package rayapp

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func Test(tmplName, buildFile string) error {
	runner := NewWorkspaceTestConfig(tmplName, buildFile)
	if err := runner.Run(); err != nil {
		return fmt.Errorf("test failed: %w", err)
	}
	return nil
}

const testCmd = "pip install nbmake==1.5.5 pytest==9.0.2 && pytest --nbmake . -s -vv"

const workspaceStartWaitTime = 30 * time.Second

// WorkspaceTestConfig contains all the details to test a workspace.
type WorkspaceTestConfig struct {
	tmplName      string
	buildFile     string
	workspaceName string
	workspaceID   string
	configFile    string
	computeConfig string
	imageURI      string
	rayVersion    string
	template      *Template
}

// NewWorkspaceTestConfig creates a new WorkspaceTestConfig for a template.
func NewWorkspaceTestConfig(tmplName, buildFile string) *WorkspaceTestConfig {
	return &WorkspaceTestConfig{tmplName: tmplName, buildFile: buildFile}
}

// Run creates an empty workspace and copies the template to it.
func (wtc *WorkspaceTestConfig) Run() error {
	// init anyscale cli
	anyscaleCLI := NewAnyscaleCLI()

	// read build file and get template details
	tmpls, err := readTemplates(wtc.buildFile)
	if err != nil {
		return fmt.Errorf("read templates failed: %w", err)
	}

	// Get the directory containing the build file to resolve relative paths
	buildDir := filepath.Dir(wtc.buildFile)

	for _, tmpl := range tmpls {
		if tmpl.Name == wtc.tmplName {
			wtc.template = tmpl
			// Resolve template directory relative to build file
			wtc.template.Dir = filepath.Join(buildDir, tmpl.Dir)
			break
		}
	}

	if wtc.template == nil {
		return fmt.Errorf("template %q not found in %s", wtc.tmplName, wtc.buildFile)
	}

	// Parse compute config name from template's AWS config path and create if needed
	if awsConfigPath, ok := wtc.template.ComputeConfig["AWS"]; ok {
		wtc.computeConfig = parseComputeConfigName(awsConfigPath)
		// Resolve compute config path relative to build file directory
		resolvedConfigPath := filepath.Join(buildDir, awsConfigPath)
		// Create compute config if it doesn't already exist
		if _, err := anyscaleCLI.CreateComputeConfig(wtc.computeConfig, resolvedConfigPath); err != nil {
			return fmt.Errorf("create compute config failed: %w", err)
		}
	}

	// generate workspace name
	workspaceName := wtc.tmplName + "-" + time.Now().Format("20060102150405")
	wtc.workspaceName = workspaceName

	// create empty workspace
	workspaceID, err := anyscaleCLI.createEmptyWorkspace(wtc)
	if err != nil {
		return fmt.Errorf("create empty workspace failed: %w", err)
	}
	wtc.workspaceID = workspaceID

	if err := anyscaleCLI.startWorkspace(wtc); err != nil {
		return fmt.Errorf("start workspace failed: %w", err)
	}

	if _, err := anyscaleCLI.waitForWorkspaceState(wtc.workspaceName, StateRunning); err != nil {
		return fmt.Errorf("wait for workspace running state failed: %w", err)
	}

	// state, err := anyscaleCLI.getWorkspaceStatus(wtc.workspaceName)
	// if err != nil {
	// 	return fmt.Errorf("get workspace state failed: %w", err)
	// }

	// for !strings.Contains(state, StateRunning.String()) {
	// 	state, err = anyscaleCLI.getWorkspaceStatus(wtc.workspaceName)
	// 	if err != nil {
	// 		return fmt.Errorf("get workspace status failed: %w, retrying...", err)
	// 	}
	// 	time.Sleep(workspaceStartWaitTime)
	// 	fmt.Println("workspace state: ", state)
	// }

	// Create temp directory for the zip file
	templateZipDir, err := os.MkdirTemp("", "template_zip")
	if err != nil {
		return fmt.Errorf("create temp directory failed: %w", err)
	}
	defer os.RemoveAll(templateZipDir) // clean up temp directory after push

	// Zip template directory to the temp directory
	zipFileName := filepath.Join(templateZipDir, wtc.tmplName+".zip")
	if err := zipDirectory(wtc.template.Dir, zipFileName); err != nil {
		return fmt.Errorf("zip template directory failed: %w", err)
	}

	if err := anyscaleCLI.pushFolderToWorkspace(wtc.workspaceName, templateZipDir); err != nil {
		return fmt.Errorf("push zip to workspace failed: %w", err)
	}

	if err := anyscaleCLI.runCmdInWorkspace(wtc, "unzip -o "+wtc.tmplName+".zip"); err != nil {
		return fmt.Errorf("run_command failed: %w", err)
	}

	// run test in workspace
	if err := anyscaleCLI.runCmdInWorkspace(wtc, testCmd); err != nil {
		return fmt.Errorf("run_command failed: %w", err)
	}

	// terminate workspace
	if err := anyscaleCLI.terminateWorkspace(wtc.workspaceName); err != nil {
		return fmt.Errorf("terminate workspace failed: %w", err)
	}

	if _, err := anyscaleCLI.waitForWorkspaceState(wtc.workspaceName, StateTerminated); err != nil {
		return fmt.Errorf("wait for workspace terminated state failed: %w", err)
	}

	if err := anyscaleCLI.deleteWorkspaceByID(wtc.workspaceID); err != nil {
		return fmt.Errorf("delete workspace failed: %w", err)
	}

	return nil
}
