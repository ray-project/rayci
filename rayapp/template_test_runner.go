package rayapp

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const testCmd = "pip install nbmake==1.5.5 pytest==9.0.2 && pytest --nbmake . -s -vv"

// WorkspaceTestConfig contains all the details to test a workspace.
type WorkspaceTestConfig struct {
	anyscaleAPI   *anyscaleAPI
	anyscaleCLI   *AnyscaleCLI
	tmplName      string
	buildDir      string
	workspaceName string
	workspaceID   string
	configFile    string
	computeConfig string
	imageURI      string
	rayVersion    string
	template      *Template
	success       bool
	errs          []error
}

// NewWorkspaceTestConfig creates a new WorkspaceTestConfig for a template.
func NewWorkspaceTestConfig(tmplName string) *WorkspaceTestConfig {
	return &WorkspaceTestConfig{tmplName: tmplName, success: false, errs: nil}
}

func RunAllTemplateTests(buildFile string) error {
	cli := NewAnyscaleCLI()
	host, token := os.Getenv("ANYSCALE_HOST"), os.Getenv("ANYSCALE_CLI_TOKEN")
	api, err := newAnyscaleAPI(host, token)
	if err != nil {
		return fmt.Errorf("new anyscale api failed: %w", err)
	}
	return runTemplateTestsWithFilter(buildFile, nil, cli, api)
}

func RunTemplateTest(tmplName, buildFile string) error {
	cli := NewAnyscaleCLI()
	host, token := os.Getenv("ANYSCALE_HOST"), os.Getenv("ANYSCALE_CLI_TOKEN")
	api, err := newAnyscaleAPI(host, token)
	if err != nil {
		return fmt.Errorf("new anyscale api failed: %w", err)
	}
	return runTemplateTestsWithFilter(buildFile, func(tmpl *Template) bool {
		return tmpl.Name == tmplName
	}, cli, api)
}

func Probe(tmplName string) (errors []error) {
	anyscaleCLI := NewAnyscaleCLI()
	anyscaleAPI, err := newAnyscaleAPI(os.Getenv("ANYSCALE_HOST"), os.Getenv("ANYSCALE_CLI_TOKEN"))
	if err != nil {
		errors = append(errors, fmt.Errorf("new anyscale api failed: %w", err))
		return errors
	}

	cloudInfo, err := anyscaleCLI.GetDefaultCloud()
	if err != nil {
		errors = append(errors, fmt.Errorf("get default cloud failed: %w", err))
		return errors
	}

	projectInfo, err := anyscaleCLI.GetDefaultProject(cloudInfo.ID)
	if err != nil {
		errors = append(errors, fmt.Errorf("get default project failed: %w", err))
		return errors
	}

	result, err := anyscaleAPI.launchTemplateInWorkspace(cloudInfo.ID, projectInfo.ID, tmplName)
	if err != nil {
		errors = append(errors, fmt.Errorf("launch template in workspace failed: %w", err))
		return errors
	}

	workspaceName, okName := result["name"].(string)
	workspaceID, okID := result["id"].(string)
	if !okName || !okID {
		errors = append(errors, fmt.Errorf("unexpected response format: missing name or id"))
		return errors
	}

	defer func() {
		if err := cleanupWorkspace(
			anyscaleCLI, anyscaleAPI, workspaceName, workspaceID,
		); err != nil {
			errors = append(errors, err)
		}
	}()

	if _, err := anyscaleCLI.waitForWorkspaceState(workspaceName, StateRunning); err != nil {
		errors = append(errors, fmt.Errorf("wait for workspace running state failed: %w", err))
		return errors
	}
	log.Println("Workspace launched successfully:", workspaceName)
	return errors
}

func NewWorkspaceTestConfig(
	t *Template,
	anyscaleCLI *AnyscaleCLI,
	anyscaleAPI *anyscaleAPI,
	buildDir string,
) *WorkspaceTestConfig {
	tmplCopy := *t
	tmplCopy.Dir = filepath.Join(buildDir, t.Dir)
	return &WorkspaceTestConfig{
		tmplName:      t.Name,
		anyscaleCLI:   anyscaleCLI,
		anyscaleAPI:   anyscaleAPI,
		success:       false,
		errs:          nil,
		template:      &tmplCopy,
		buildDir:      buildDir,
		workspaceName: fmt.Sprintf("%s-%s", t.Name, time.Now().Format("20060102150405")),
	}
}

func RunAllTemplateTests(buildFile string) error {
	cli := NewAnyscaleCLI()
	host, token := os.Getenv("ANYSCALE_HOST"), os.Getenv("ANYSCALE_CLI_TOKEN")
	api, err := newAnyscaleAPI(host, token)
	if err != nil {
		return fmt.Errorf("new anyscale api failed: %w", err)
	}
	return runTemplateTestsWithFilter(buildFile, nil, cli, api)
}

func RunTemplateTest(tmplName, buildFile string) error {
	cli := NewAnyscaleCLI()
	host, token := os.Getenv("ANYSCALE_HOST"), os.Getenv("ANYSCALE_CLI_TOKEN")
	api, err := newAnyscaleAPI(host, token)
	if err != nil {
		return fmt.Errorf("new anyscale api failed: %w", err)
	}
	return runTemplateTestsWithFilter(buildFile, func(tmpl *Template) bool {
		return tmpl.Name == tmplName
	}, cli, api)
}

func runTemplateTestsWithFilter(
	buildFile string,
	filter func(tmpl *Template) bool,
	cli *AnyscaleCLI,
	api *anyscaleAPI,
) error {
	tmpls, err := readTemplates(buildFile)
	if err != nil {
		return fmt.Errorf("read templates failed: %w", err)
	}

	buildDir := filepath.Dir(buildFile)

	var filteredTmpls []*Template
	for _, t := range tmpls {
		if filter == nil || filter(t) {
			filteredTmpls = append(filteredTmpls, t)
		}
	}
	if len(filteredTmpls) == 0 {
		return fmt.Errorf("no templates to test")
	}

	var failed []string
	for _, t := range filteredTmpls {
		c := NewWorkspaceTestConfig(t, cli, api, buildDir)

		log.Println("Testing template:", c.tmplName)
		c.Run()

		log.Println("Success:", c.success)
		if !c.success {
			log.Println("Error:", c.errs)
			failed = append(failed, fmt.Sprintf("%s: %v", c.tmplName, c.errs))
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("test failed for templates:\n%s", strings.Join(failed, "\n"))
	}

	return nil
}

// Run creates an empty workspace and copies the template to it.
func (wtc *WorkspaceTestConfig) Run() (errors []error) {

	// init anyscale cli
	anyscaleCLI := NewAnyscaleCLI()
	anyscaleAPI, err := newAnyscaleAPI(os.Getenv("ANYSCALE_HOST"), os.Getenv("ANYSCALE_CLI_TOKEN"))
	if err != nil {
		errors = append(errors, fmt.Errorf("new anyscale api failed: %w", err))
		return errors
	}

	// Parse compute config name from template's AWS config path and create if needed
	if awsConfigPath, ok := wtc.template.ComputeConfig["AWS"]; ok {
		wtc.computeConfig = generateComputeConfigName(awsConfigPath)
		// Resolve compute config path relative to build file directory
		resolvedConfigPath := filepath.Join(wtc.buildDir, awsConfigPath)
		// Create compute config if it doesn't already exist
		if err := anyscaleCLI.CreateComputeConfig(
			wtc.computeConfig,
			resolvedConfigPath,
		); err != nil {
			errors = append(errors, fmt.Errorf("create compute config failed: %w", err))
			return errors
		}
	}

	// generate workspace name
	workspaceName := wtc.tmplName + "-" + time.Now().Format("20060102150405")
	wtc.workspaceName = workspaceName

	// create empty workspace
	err = anyscaleCLI.createEmptyWorkspace(wtc)
	if err != nil {
		errors = append(errors, fmt.Errorf("create empty workspace failed: %w", err))
		return errors
	}

	wtc.workspaceID, err = anyscaleCLI.getWorkspaceID(wtc.workspaceName)
	if err != nil {
		errors = append(errors, fmt.Errorf("get workspace ID failed: %w", err))
		return errors
	}

	defer func() {
		if err := cleanupWorkspace(anyscaleCLI, anyscaleAPI, wtc.workspaceName, wtc.workspaceID); err != nil {
			errors = append(errors, err)
		}
	}()

	if err := anyscaleCLI.startWorkspace(wtc.workspaceName); err != nil {
		errors = append(errors, fmt.Errorf("start workspace failed: %w", err))
		return errors
	}

	if _, err := anyscaleCLI.waitForWorkspaceState(wtc.workspaceName, StateRunning); err != nil {
		errors = append(errors, fmt.Errorf("wait for workspace running state failed: %w", err))
		return errors
	}

	// Create temp directory for the zip file
	templateZipDir, err := os.MkdirTemp("", "template_zip")
	if err != nil {
		errors = append(errors, fmt.Errorf("create temp directory failed: %w", err))
		return errors
	}
	defer os.RemoveAll(templateZipDir) // clean up temp directory after push

	// Zip template directory to the temp directory
	zipFileName := filepath.Join(templateZipDir, wtc.tmplName+".zip")
	if err := zipDirectory(wtc.template.Dir, zipFileName); err != nil {
		errors = append(errors, fmt.Errorf("zip template directory failed: %w", err))
		return errors
	}

	if err := anyscaleCLI.pushFolderToWorkspace(wtc.workspaceName, templateZipDir); err != nil {
		errors = append(errors, fmt.Errorf("push zip to workspace failed: %w", err))
		return errors
	}

	if err := anyscaleCLI.runCmdInWorkspace(
		wtc.workspaceName,
		"unzip -o "+wtc.tmplName+".zip",
	); err != nil {
		errors = append(errors, fmt.Errorf("run_command failed: %w", err))
		return errors
	}

	// run test in workspace
	if err := anyscaleCLI.runCmdInWorkspace(wtc.workspaceName, testCmd); err != nil {
		errors = append(errors, fmt.Errorf("run_command failed: %w", err))
		return errors
	}

	return errors
}

// Run creates an empty workspace, copies the template to it, and runs tests.
// It sets c.errs and c.success based on the outcome.
func (c *WorkspaceTestConfig) Run() {
	c.errs = nil
	c.success = false

	defer func() {
		c.success = len(c.errs) == 0
		if c.success {
			log.Println("Test completed successfully")
		} else {
			log.Println("Test completed with errors")
		}
	}()

	// Currently only AWS compute configs are supported for workspace testing.
	if awsConfigPath, ok := c.template.ComputeConfig["AWS"]; ok {
		c.computeConfig = generateComputeConfigName(awsConfigPath)
		resolvedConfigPath := filepath.Join(c.buildDir, awsConfigPath)
		if err := c.anyscaleCLI.CreateComputeConfig(
			c.computeConfig,
			resolvedConfigPath,
		); err != nil {
			c.errs = append(c.errs, fmt.Errorf("create compute config failed: %w", err))
			return
		}
	}

	if err := c.anyscaleCLI.createEmptyWorkspace(c); err != nil {
		c.errs = append(c.errs, fmt.Errorf("create empty workspace failed: %w", err))
		return
	}
	workspaceID, err := c.anyscaleCLI.getWorkspaceID(c.workspaceName)
	if err != nil {
		c.errs = append(c.errs, fmt.Errorf("get workspace ID failed: %w", err))
		return
	}
	c.workspaceID = workspaceID

	defer func() {
		log.Println("Cleaning up workspace...")
		if err := c.anyscaleCLI.terminateWorkspace(c.workspaceName); err != nil {
			c.errs = append(c.errs, fmt.Errorf("terminate workspace failed: %w", err))
			return
		}
		if _, err := c.anyscaleCLI.waitForWorkspaceState(
			c.workspaceName,
			StateTerminated,
		); err != nil {
			c.errs = append(
				c.errs,
				fmt.Errorf("wait for workspace terminated state failed: %w", err),
			)
			return
		}
		log.Println("Terminated workspace:", c.workspaceID)

		if err := c.anyscaleAPI.deleteWorkspaceByID(c.workspaceID); err != nil {
			c.errs = append(c.errs, fmt.Errorf("delete workspace failed: %w", err))
			return
		}
		log.Println("Deleted workspace:", c.workspaceID)
	}()

	if err := c.anyscaleCLI.startWorkspace(c.workspaceName); err != nil {
		c.errs = append(c.errs, fmt.Errorf("start workspace failed: %w", err))
		return
	}

	if _, err := c.anyscaleCLI.waitForWorkspaceState(
		c.workspaceName, StateRunning,
	); err != nil {
		c.errs = append(
			c.errs, fmt.Errorf("wait for workspace running state failed: %w", err),
		)
		return
	}

	templateZipDir, err := os.MkdirTemp("", "template_zip")
	if err != nil {
		c.errs = append(c.errs, fmt.Errorf("create temp directory failed: %w", err))
		return
	}
	defer os.RemoveAll(templateZipDir)

	zipFileName := filepath.Join(templateZipDir, fmt.Sprintf("%s.zip", c.tmplName))
	if err := zipDirectory(c.template.Dir, zipFileName); err != nil {
		c.errs = append(c.errs, fmt.Errorf("zip template directory failed: %w", err))
		return
	}

	if err := c.anyscaleCLI.pushFolderToWorkspace(
		c.workspaceName, templateZipDir,
	); err != nil {
		c.errs = append(c.errs, fmt.Errorf("push zip to workspace failed: %w", err))
		return
	}

	if err := c.anyscaleCLI.runCmdInWorkspace(
		c.workspaceName,
		fmt.Sprintf("unzip -o %s.zip", c.tmplName),
	); err != nil {
		c.errs = append(c.errs, fmt.Errorf("unzip template failed: %w", err))
		return
	}

	if err := c.anyscaleCLI.runCmdInWorkspace(c.workspaceName, testCmd); err != nil {
		c.errs = append(c.errs, fmt.Errorf("test command failed: %w", err))
		return
	}
}

func cleanupWorkspace(anyscaleCLI *AnyscaleCLI, anyscaleAPI *anyscaleAPI, workspaceName, workspaceID string) error {
	log.Println("Cleaning up workspace...")
	if err := anyscaleCLI.terminateWorkspace(workspaceName); err != nil {
		return fmt.Errorf("terminate workspace failed: %w", err)
	}
	if _, err := anyscaleCLI.waitForWorkspaceState(
		workspaceName,
		StateTerminated,
	); err != nil {
		return fmt.Errorf("wait for workspace terminated state failed: %w", err)
	}
	if err := anyscaleAPI.deleteWorkspaceByID(workspaceID); err != nil {
		return fmt.Errorf("delete workspace failed: %w", err)
	}
	return nil
}