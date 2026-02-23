package rayapp

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
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
	return runTemplateTestsWithFilter(buildFile, nil)
}

func RunTemplateTest(tmplName, buildFile string) error {
	return runTemplateTestsWithFilter(buildFile, func(tmpl *Template) bool {
		return tmpl.Name == tmplName
	})
}

func runTemplateTestsWithFilter(buildFile string, filter func(tmpl *Template) bool) error {
	// read build file and get template details
	tmpls, err := readTemplates(buildFile)
	if err != nil {
		return fmt.Errorf("read templates failed: %w", err)
	}

	// Get the directory containing the build file to resolve relative paths
	buildDir := filepath.Dir(buildFile)

	filteredTmpls := slices.Collect(func(yield func(*Template) bool) {
		for _, t := range tmpls {
			if filter != nil && !filter(t) {
				continue
			}
			if !yield(t) {
				return
			}
		}
	})
	if len(filteredTmpls) == 0 {
		return fmt.Errorf("no templates to test")
	}

	anyscaleCLI := NewAnyscaleCLI()
	anyscaleAPI, err := newAnyscaleAPI(
		os.Getenv("ANYSCALE_HOST"), os.Getenv("ANYSCALE_CLI_TOKEN"),
	)
	if err != nil {
		return fmt.Errorf("new anyscale api failed: %w", err)
	}

	var testConfigs []*WorkspaceTestConfig
	for _, t := range filteredTmpls {
		c := NewWorkspaceTestConfig(t.Name)
		c.anyscaleCLI = anyscaleCLI
		c.anyscaleAPI = anyscaleAPI
		c.template = t
		c.buildDir = buildDir
		c.template.Dir = filepath.Join(buildDir, t.Dir)
		testConfigs = append(testConfigs, c)
	}

	for _, c := range testConfigs {
		log.Println("Testing template:", c.template.Name)
		c.Run()
	}

	var failed []string
	for _, c := range testConfigs {
		log.Println("Template:", c.template.Name)
		log.Println("Success:", c.success)
		if !c.success {
			log.Println("Error:", c.errs)
			failed = append(failed, fmt.Sprintf("%s: %v", c.template.Name, c.errs))
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("test failed for templates:\n%s", strings.Join(failed, "\n"))
	}

	return nil
}

// Run creates an empty workspace, copies the template to it, and runs tests.
// It sets c.errs and c.success based on the outcome.
func (c *WorkspaceTestConfig) Run() {
	c.errs = nil
	c.success = false

	defer func() {
		log.Println("Test completed successfully")
		c.success = len(c.errs) == 0
	}()

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

	workspaceName := c.tmplName + "-" + time.Now().Format("20060102150405")
	c.workspaceName = workspaceName

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

	zipFileName := filepath.Join(templateZipDir, c.tmplName+".zip")
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
		c.errs = append(c.errs, fmt.Errorf("run_command failed: %w", err))
		return
	}

	if err := c.anyscaleCLI.runCmdInWorkspace(c.workspaceName, testCmd); err != nil {
		c.errs = append(c.errs, fmt.Errorf("run_command failed: %w", err))
		return
	}
}
