package rayapp

import (
	"fmt"
	"os"
	"strings"
	"time"
)

func Test(tmplName, buildFile string) error {
	runner := NewWorkspaceTestConfig(tmplName, buildFile)
	if err := runner.Run(); err != nil {
		return fmt.Errorf("test failed: %w", err)
	}
	return nil
}

const testCmd = "pip install nbmake==1.5.5 pytest==7.4.0 && pytest --nbmake . -s -vv"

const workspaceStartWaitTime = 30 * time.Second
// WorkspaceTestConfig contains all the details to test a workspace.
type WorkspaceTestConfig struct {
	tmplName      string
	buildFile     string
	workspaceName string
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
	anyscaleCLI := NewAnyscaleCLI(os.Getenv("ANYSCALE_CLI_TOKEN"))

	// read build file and get template details
	tmpls, err := readTemplates(wtc.buildFile)
	if err != nil {
		return fmt.Errorf("read templates failed: %w", err)
	}

	for _, tmpl := range tmpls {
		if tmpl.Name == wtc.tmplName {
			wtc.template = tmpl
			break
		}
	}

	// generate workspace name
	workspaceName := wtc.tmplName + "-" + time.Now().Format("20060102150405")
	wtc.workspaceName = workspaceName

	// create empty workspace
	if err := anyscaleCLI.createEmptyWorkspace(wtc); err != nil {
		return fmt.Errorf("create empty workspace failed: %w", err)
	}

	if err := anyscaleCLI.startWorkspace(wtc); err != nil {
		return fmt.Errorf("start workspace failed: %w", err)
	}

	state, err := anyscaleCLI.getWorkspaceStatus(wtc.workspaceName)
	if err != nil {
		return fmt.Errorf("get workspace state failed: %w", err)
	}

	for !strings.Contains(state, StateRunning.String()) {
		state, err = anyscaleCLI.getWorkspaceStatus(wtc.workspaceName)
		if err != nil {
			return fmt.Errorf("get workspace status failed: %w, retrying...", err)
		}
		time.Sleep(workspaceStartWaitTime)
		fmt.Println("workspace state: ", state)
	}

	// copy template to workspace
	if err := anyscaleCLI.copyTemplateToWorkspace(wtc); err != nil {
		return fmt.Errorf("copy template to workspace failed: %w", err)
	}

	// run test in workspace
	if err := anyscaleCLI.runCmdInWorkspace(wtc, testCmd); err != nil {
		return fmt.Errorf("run test in workspace failed: %w", err)
	}

	// terminate workspace
	if err := anyscaleCLI.terminateWorkspace(tr.workspaceName); err != nil {
		return fmt.Errorf("terminate workspace failed: %w", err)
	}

	return nil
}
