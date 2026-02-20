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

func Probe(tmplName string, buildFile string) error {
	anyscaleCLI := NewAnyscaleCLI()
	anyscaleAPI, err := newAnyscaleAPI()
	if err != nil {
		return fmt.Errorf("new anyscale api failed: %w", err)
	}

	cloudInfo, err := anyscaleCLI.GetDefaultCloud()
	if err != nil {
		return fmt.Errorf("get default cloud failed: %w", err)
	}

	projectInfo, err := anyscaleCLI.GetDefaultProject(cloudInfo.ID)
	if err != nil {
		return fmt.Errorf("get default project failed: %w", err)
	}

	result, err := anyscaleAPI.LaunchTemplateInWorkspace(cloudInfo.ID, projectInfo.ID, tmplName)
	if err != nil {
		return fmt.Errorf("launch template in workspace failed: %w", err)
	}
	workspaceName := result["name"].(string)
	workspaceID := result["id"].(string)

	defer func() {
		if err := cleanupWorkspace(anyscaleCLI, anyscaleAPI, workspaceName, workspaceID); err != nil {
			log.Printf("cleanup failed: %v", err)
		}
	}()

	if _, err := anyscaleCLI.waitForWorkspaceState(workspaceName, StateRunning); err != nil {
		return fmt.Errorf("wait for workspace running state failed: %w", err)
	}

	fmt.Println("Workspace launched successfully:", workspaceName)
	return nil
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
	anyscaleAPI, err := newAnyscaleAPI()
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

func cleanupWorkspace(anyscaleCLI *AnyscaleCLI, anyscaleAPI *AnyscaleAPI, workspaceName, workspaceID string) error {
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
	if err := anyscaleAPI.DeleteWorkspaceByID(workspaceID); err != nil {
		return fmt.Errorf("delete workspace failed: %w", err)
	}
	return nil
}
