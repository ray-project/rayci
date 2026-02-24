package rayapp

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
		if filter != nil && !filter(t) {
			continue
		}
		if t.Test == nil {
			log.Printf("Template %s has no test configuration, skipping", t.Name)
			continue
		}
		filteredTmpls = append(filteredTmpls, t)
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
			c.workspaceName, StateTerminated,
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

	if _, err := c.anyscaleCLI.waitForWorkspaceState(c.workspaceName, StateRunning); err != nil {
		c.errs = append(c.errs, fmt.Errorf("wait for workspace running state failed: %w", err))
		return
	}

	templateZipDir, err := os.MkdirTemp("", "template_zip")
	if err != nil {
		c.errs = append(c.errs, fmt.Errorf("create temp directory failed: %w", err))
		return
	}
	defer os.RemoveAll(templateZipDir)

	templateZipFileName := filepath.Join(templateZipDir, fmt.Sprintf("%s.zip", c.tmplName))
	if err := zipDirectory(c.template.Dir, templateZipFileName); err != nil {
		c.errs = append(c.errs, fmt.Errorf("zip template directory failed: %w", err))
		return
	}

	if err := c.anyscaleCLI.pushFolderToWorkspace(c.workspaceName, templateZipDir); err != nil {
		c.errs = append(c.errs, fmt.Errorf("push template zip to workspace failed: %w", err))
		return
	}

	if err := c.anyscaleCLI.runCmdInWorkspace(
		c.workspaceName, fmt.Sprintf("unzip -o %s.zip", c.tmplName),
	); err != nil {
		c.errs = append(c.errs, fmt.Errorf("unzip template failed: %w", err))
		return
	}

	// If tests_path is provided, zip and push test folder
	if c.template.Test.TestsPath != "" {
		testsPath := filepath.Join(c.buildDir, c.template.Test.TestsPath)
		testZipDir, err := os.MkdirTemp("", "test_zip")
		if err != nil {
			c.errs = append(c.errs, fmt.Errorf("create test temp directory failed: %w", err))
			return
		}
		defer os.RemoveAll(testZipDir)

		testZipFileName := filepath.Join(testZipDir, "tests.zip")
		if err := zipDirectory(testsPath, testZipFileName); err != nil {
			c.errs = append(c.errs, fmt.Errorf("zip test directory failed: %w", err))
			return
		}

		// Push test zip to workspace
		if err := c.anyscaleCLI.pushFolderToWorkspace(c.workspaceName, testZipDir); err != nil {
			c.errs = append(c.errs, fmt.Errorf("push test zip to workspace failed: %w", err))
			return
		}

		// Unzip test folder in workspace
		if err := c.anyscaleCLI.runCmdInWorkspace(
			c.workspaceName, "unzip -o tests.zip",
		); err != nil {
			c.errs = append(c.errs, fmt.Errorf("unzip tests in workspace failed: %w", err))
			return
		}
	}

	// Run test command from test configuration
	testCommand := fmt.Sprintf(
		"timeout %d bash -c '%s'", c.template.Test.TimeoutInSec, c.template.Test.Command,
	)
	if err := c.anyscaleCLI.runCmdInWorkspace(c.workspaceName, testCommand); err != nil {
		c.errs = append(c.errs, fmt.Errorf("run test command failed: %w", err))
		return
	}
}
