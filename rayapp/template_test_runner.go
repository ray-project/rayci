package rayapp

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const testCmd = "pip install nbmake==1.5.5 pytest==9.0.2 && " +
	"pytest --nbmake . -s -vv"

// workspaceLauncher sets up a workspace for testing.
type workspaceLauncher interface {
	// setup creates and prepares a workspace for test execution.
	// It sets c.workspaceName and c.workspaceID, and leaves
	// the workspace in a running state ready for tests.
	setup(c *WorkspaceTestConfig) error
}

// emptyWorkspaceLauncher creates an empty workspace, starts it,
// and pushes template files into it.
type emptyWorkspaceLauncher struct{}

func (l *emptyWorkspaceLauncher) setup(
	c *WorkspaceTestConfig,
) error {
	if awsConfigPath, ok := c.template.ComputeConfig["AWS"]; ok {
		c.computeConfig = generateComputeConfigName(awsConfigPath)
		resolvedPath := filepath.Join(c.buildDir, awsConfigPath)
		if err := c.anyscaleCLI.CreateComputeConfig(
			c.computeConfig, resolvedPath,
		); err != nil {
			return fmt.Errorf(
				"create compute config failed: %w", err,
			)
		}
	}

	if err := c.anyscaleCLI.createEmptyWorkspace(c); err != nil {
		return fmt.Errorf(
			"create empty workspace failed: %w", err,
		)
	}
	workspaceID, err := c.anyscaleCLI.getWorkspaceID(
		c.workspaceName,
	)
	if err != nil {
		return fmt.Errorf("get workspace ID failed: %w", err)
	}
	c.workspaceID = workspaceID

	if err := c.anyscaleCLI.startWorkspace(
		c.workspaceName,
	); err != nil {
		return fmt.Errorf("start workspace failed: %w", err)
	}

	if _, err := c.anyscaleCLI.waitForWorkspaceState(
		c.workspaceName, StateRunning,
	); err != nil {
		return fmt.Errorf(
			"wait for workspace running state failed: %w", err,
		)
	}

	templateZipDir, err := os.MkdirTemp("", "template_zip")
	if err != nil {
		return fmt.Errorf(
			"create temp directory failed: %w", err,
		)
	}
	defer os.RemoveAll(templateZipDir)

	zipFileName := filepath.Join(
		templateZipDir,
		fmt.Sprintf("%s.zip", c.tmplName),
	)
	if err := zipDirectory(c.template.Dir, zipFileName); err != nil {
		return fmt.Errorf(
			"zip template directory failed: %w", err,
		)
	}

	if err := c.anyscaleCLI.pushFolderToWorkspace(
		c.workspaceName, templateZipDir,
	); err != nil {
		return fmt.Errorf(
			"push zip to workspace failed: %w", err,
		)
	}

	if err := c.anyscaleCLI.runCmdInWorkspace(
		c.workspaceName,
		fmt.Sprintf("unzip -o %s.zip", c.tmplName),
	); err != nil {
		return fmt.Errorf("unzip template failed: %w", err)
	}

	return nil
}

// templateWorkspaceLauncher launches a workspace from a template
// via the Anyscale API.
type templateWorkspaceLauncher struct{}

func (l *templateWorkspaceLauncher) setup(
	c *WorkspaceTestConfig,
) error {
	cloudInfo, err := c.anyscaleCLI.GetDefaultCloud()
	if err != nil {
		return fmt.Errorf(
			"get default cloud failed: %w", err,
		)
	}

	projectInfo, err := c.anyscaleCLI.GetDefaultProject(
		cloudInfo.ID,
	)
	if err != nil {
		return fmt.Errorf(
			"get default project failed: %w", err,
		)
	}

	result, err := c.anyscaleAPI.launchTemplateInWorkspace(
		cloudInfo.ID, projectInfo.ID, c.tmplName,
	)
	if err != nil {
		return fmt.Errorf(
			"launch template in workspace failed: %w", err,
		)
	}

	workspaceName, okName := result["name"].(string)
	workspaceID, okID := result["id"].(string)
	if !okName || !okID {
		return fmt.Errorf(
			"unexpected response format: missing name or id",
		)
	}
	c.workspaceName = workspaceName
	c.workspaceID = workspaceID

	if _, err := c.anyscaleCLI.waitForWorkspaceState(
		c.workspaceName, StateRunning,
	); err != nil {
		return fmt.Errorf(
			"wait for workspace running state failed: %w",
			err,
		)
	}

	return nil
}

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
	launcher      workspaceLauncher
	success       bool
	errs          []error
}

// Probe launches a template into a workspace, runs tests, and
// cleans up.
func Probe(tmplName string) error {
	cli := NewAnyscaleCLI()
	api, err := newAnyscaleAPI(
		os.Getenv("ANYSCALE_HOST"),
		os.Getenv("ANYSCALE_CLI_TOKEN"),
	)
	if err != nil {
		return fmt.Errorf("new anyscale api failed: %w", err)
	}
	return probe(tmplName, cli, api)
}

func probe(
	tmplName string, cli *AnyscaleCLI, api *anyscaleAPI,
) error {
	c := &WorkspaceTestConfig{
		tmplName:    tmplName,
		anyscaleCLI: cli,
		anyscaleAPI: api,
		launcher:    &templateWorkspaceLauncher{},
	}
	c.Run()
	if !c.success {
		return errors.Join(c.errs...)
	}
	return nil
}

// NewWorkspaceTestConfig creates a new WorkspaceTestConfig that
// uses an empty workspace to test a template.
func NewWorkspaceTestConfig(
	t *Template,
	anyscaleCLI *AnyscaleCLI,
	anyscaleAPI *anyscaleAPI,
	buildDir string,
) *WorkspaceTestConfig {
	tmplCopy := *t
	tmplCopy.Dir = filepath.Join(buildDir, t.Dir)
	return &WorkspaceTestConfig{
		tmplName:    t.Name,
		anyscaleCLI: anyscaleCLI,
		anyscaleAPI: anyscaleAPI,
		success:     false,
		errs:        nil,
		template:    &tmplCopy,
		buildDir:    buildDir,
		launcher:    &emptyWorkspaceLauncher{},
		workspaceName: fmt.Sprintf(
			"%s-%s", t.Name, time.Now().Format("20060102150405"),
		),
	}
}

func RunAllTemplateTests(buildFile string) error {
	cli := NewAnyscaleCLI()
	host, token := os.Getenv("ANYSCALE_HOST"),
		os.Getenv("ANYSCALE_CLI_TOKEN")
	api, err := newAnyscaleAPI(host, token)
	if err != nil {
		return fmt.Errorf("new anyscale api failed: %w", err)
	}
	return runTemplateTestsWithFilter(buildFile, nil, cli, api)
}

func RunTemplateTest(tmplName, buildFile string) error {
	cli := NewAnyscaleCLI()
	host, token := os.Getenv("ANYSCALE_HOST"),
		os.Getenv("ANYSCALE_CLI_TOKEN")
	api, err := newAnyscaleAPI(host, token)
	if err != nil {
		return fmt.Errorf("new anyscale api failed: %w", err)
	}
	return runTemplateTestsWithFilter(
		buildFile,
		func(tmpl *Template) bool {
			return tmpl.Name == tmplName
		},
		cli, api,
	)
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
			failed = append(
				failed,
				fmt.Sprintf("%s: %v", c.tmplName, c.errs),
			)
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf(
			"test failed for templates:\n%s",
			strings.Join(failed, "\n"),
		)
	}

	return nil
}

func (c *WorkspaceTestConfig) cleanup() {
	log.Println("Cleaning up workspace...")
	if err := c.anyscaleCLI.terminateWorkspace(
		c.workspaceName,
	); err != nil {
		c.errs = append(
			c.errs,
			fmt.Errorf("terminate workspace failed: %w", err),
		)
		return
	}
	if _, err := c.anyscaleCLI.waitForWorkspaceState(
		c.workspaceName, StateTerminated,
	); err != nil {
		c.errs = append(
			c.errs,
			fmt.Errorf(
				"wait for workspace terminated state failed: %w",
				err,
			),
		)
		return
	}
	log.Println("Terminated workspace:", c.workspaceID)

	if err := c.anyscaleAPI.deleteWorkspaceByID(
		c.workspaceID,
	); err != nil {
		c.errs = append(
			c.errs,
			fmt.Errorf("delete workspace failed: %w", err),
		)
		return
	}
	log.Println("Deleted workspace:", c.workspaceID)
}

// Run sets up a workspace using the configured launcher, runs
// tests, and cleans up. It sets c.errs and c.success.
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

	if err := c.launcher.setup(c); err != nil {
		c.errs = append(c.errs, err)
	}

	if c.workspaceID != "" {
		defer c.cleanup()
	}

	if len(c.errs) > 0 {
		return
	}

	if err := c.anyscaleCLI.runCmdInWorkspace(
		c.workspaceName, testCmd,
	); err != nil {
		c.errs = append(
			c.errs,
			fmt.Errorf("test command failed: %w", err),
		)
	}
}
