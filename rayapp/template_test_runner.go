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

func TestAll(buildFile string) error {
	return testWithFilter(buildFile, nil)
}

func Test(tmplName, buildFile string) error {
	return testWithFilter(buildFile, func(tmpl *Template) bool {
		return tmpl.Name == tmplName
	})
}

func testWithFilter(buildFile string, filter func(tmpl *Template) bool) error {
	// read build file and get template details
	tmpls, err := readTemplates(buildFile)
	if err != nil {
		return fmt.Errorf("read templates failed: %w", err)
	}

	// Get the directory containing the build file to resolve relative paths
	buildDir := filepath.Dir(buildFile)

	var testConfigs []*WorkspaceTestConfig

	for _, t := range tmpls {
		if filter != nil && !filter(t) {
			continue
		}
		log.Println("Testing template:", t.Name)

		runner := NewWorkspaceTestConfig(t.Name)
		runner.template = t
		runner.buildDir = buildDir
		runner.template.Dir = filepath.Join(buildDir, t.Dir)
		testConfigs = append(testConfigs, runner)
	}

	if len(testConfigs) == 0 {
		return fmt.Errorf("no templates to test")
	}

	for _, wtc := range testConfigs {
		if errs := wtc.Run(); len(errs) > 0 {
			wtc.errs = errs
			wtc.success = false
		} else {
			wtc.errs = nil
			wtc.success = true
		}
	}

	var failed []string
	for _, wtc := range testConfigs {
		log.Println("Template:", wtc.template.Name)
		log.Println("Success:", wtc.success)
		if !wtc.success {
			log.Println("Error:", wtc.errs)
			failed = append(failed, fmt.Sprintf("%s: %v", wtc.template.Name, wtc.errs))
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
	err := anyscaleCLI.createEmptyWorkspace(wtc)
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
		log.Println("Cleaning up workspace...")
		if err := anyscaleCLI.terminateWorkspace(wtc.workspaceName); err != nil {
			errors = append(errors, fmt.Errorf("terminate workspace failed: %w", err))
			return
		}
		if _, err := anyscaleCLI.waitForWorkspaceState(
			wtc.workspaceName,
			StateTerminated,
		); err != nil {
			errors = append(
				errors,
				fmt.Errorf("wait for workspace terminated state failed: %w", err),
			)
			return
		}

		if err := anyscaleCLI.deleteWorkspaceByID(wtc.workspaceID); err != nil {
			errors = append(errors, fmt.Errorf("delete workspace failed: %w", err))
			return
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

	// Create temp directory for the template zip file
	templateZipDir, err := os.MkdirTemp("", "template_zip")
	if err != nil {
		errors = append(errors, fmt.Errorf("create temp directory failed: %w", err))
		return errors
	}
	defer os.RemoveAll(templateZipDir)

	// Zip template directory to the temp directory
	templateZipFileName := filepath.Join(templateZipDir, wtc.tmplName+".zip")
	if err := zipDirectory(wtc.template.Dir, templateZipFileName); err != nil {
		errors = append(errors, fmt.Errorf("zip template directory failed: %w", err))
		return errors
	}

	// Push template zip to workspace
	if err := anyscaleCLI.pushFolderToWorkspace(wtc.workspaceName, templateZipDir); err != nil {
		errors = append(errors, fmt.Errorf("push template zip to workspace failed: %w", err))
		return errors
	}

	// Unzip template contents in workspace
	if err := anyscaleCLI.runCmdInWorkspace(
		wtc.workspaceName,
		"unzip -o "+wtc.tmplName+".zip",
	); err != nil {
		errors = append(errors, fmt.Errorf("unzip template in workspace failed: %w", err))
		return errors
	}

	// If tests_path is provided, zip and push test folder
	if wtc.template.Test.TestsPath != "" {
		testsPath := filepath.Join(wtc.buildDir, wtc.template.Test.TestsPath)
		testZipDir, err := os.MkdirTemp("", "test_zip")
		if err != nil {
			errors = append(errors, fmt.Errorf("create test temp directory failed: %w", err))
			return errors
		}
		defer os.RemoveAll(testZipDir)

		testZipFileName := filepath.Join(testZipDir, "tests.zip")
		if err := zipDirectory(testsPath, testZipFileName); err != nil {
			errors = append(errors, fmt.Errorf("zip test directory failed: %w", err))
			return errors
		}

		// Push test zip to workspace
		if err := anyscaleCLI.pushFolderToWorkspace(wtc.workspaceName, testZipDir); err != nil {
			errors = append(errors, fmt.Errorf("push test zip to workspace failed: %w", err))
			return errors
		}

		// Unzip test folder in workspace
		if err := anyscaleCLI.runCmdInWorkspace(
			wtc.workspaceName,
			"unzip -o tests.zip",
		); err != nil {
			errors = append(errors, fmt.Errorf("unzip tests in workspace failed: %w", err))
			return errors
		}
	}

	// Run test command from test configuration
	testCommand := fmt.Sprintf(
		"timeout %d bash -c '%s'", wtc.template.Test.TimeoutInSec, wtc.template.Test.Command,
	)
	if err := anyscaleCLI.runCmdInWorkspace(wtc.workspaceName, testCommand); err != nil {
		errors = append(errors, fmt.Errorf("run test command failed: %w", err))
		return errors
	}

	return errors
}
