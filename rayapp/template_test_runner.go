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
	probe         bool
	success       bool
	errs          []error
}

// newWorkspaceTestConfig creates a new WorkspaceTestConfig that uses an empty
// workspace to test a template.
func newWorkspaceTestConfig(
	t *Template,
	anyscaleCLI *AnyscaleCLI,
	anyscaleAPI *anyscaleAPI,
	buildDir string,
	probe bool,
) *WorkspaceTestConfig {
	tmplCopy := *t
	tmplCopy.Dir = filepath.Join(buildDir, t.Dir)
	return &WorkspaceTestConfig{
		tmplName:      t.Name,
		anyscaleCLI:   anyscaleCLI,
		anyscaleAPI:   anyscaleAPI,
		probe:         probe,
		success:       false,
		errs:          nil,
		template:      &tmplCopy,
		buildDir:      buildDir,
		workspaceName: fmt.Sprintf("%s-%s", t.Name, time.Now().Format("20060102150405")),
	}
}

// RunProbe launches a template into a workspace, runs tests if configured,
// and cleans up.
func RunProbe(tmplName, buildFile string) error {
	cli := NewAnyscaleCLI()
	api, err := newAnyscaleAPI(os.Getenv("ANYSCALE_HOST"), os.Getenv("ANYSCALE_CLI_TOKEN"))
	if err != nil {
		return fmt.Errorf("new anyscale api failed: %w", err)
	}
	return probe(tmplName, buildFile, cli, api)
}

func probe(tmplName, buildFile string, cli *AnyscaleCLI, api *anyscaleAPI) error {
	tmpls, err := readTemplates(buildFile)
	if err != nil {
		return fmt.Errorf("read templates failed: %w", err)
	}

	var tmpl *Template
	for _, t := range tmpls {
		if t.Name == tmplName {
			tmpl = t
			break
		}
	}
	if tmpl == nil {
		return fmt.Errorf("template %q not found in %s", tmplName, buildFile)
	}
	if tmpl.Test == nil {
		return fmt.Errorf("template %q has no test configuration", tmplName)
	}

	buildDir := filepath.Dir(buildFile)
	c := newWorkspaceTestConfig(tmpl, cli, api, buildDir, true)
	c.Run()
	if !c.success {
		return errors.Join(c.errs...)
	}
	return nil
}

func RunAllTemplateTests(buildFile, rayVersion string) error {
	cli := NewAnyscaleCLI()
	host, token := os.Getenv("ANYSCALE_HOST"), os.Getenv("ANYSCALE_CLI_TOKEN")
	api, err := newAnyscaleAPI(host, token)
	if err != nil {
		return fmt.Errorf("new anyscale api failed: %w", err)
	}
	return runTemplateTestsWithFilter(buildFile, nil, rayVersion, cli, api)
}

func RunTemplateTest(tmplName, buildFile, rayVersion string) error {
	cli := NewAnyscaleCLI()
	host, token := os.Getenv("ANYSCALE_HOST"), os.Getenv("ANYSCALE_CLI_TOKEN")
	api, err := newAnyscaleAPI(host, token)
	if err != nil {
		return fmt.Errorf("new anyscale api failed: %w", err)
	}
	return runTemplateTestsWithFilter(buildFile, func(tmpl *Template) bool {
		return tmpl.Name == tmplName
	}, rayVersion, cli, api)
}

func runTemplateTestsWithFilter(
	buildFile string,
	filter func(tmpl *Template) bool,
	rayVersion string,
	cli *AnyscaleCLI,
	api *anyscaleAPI,
) error {
	tmpls, err := readTemplates(buildFile)
	if err != nil {
		return fmt.Errorf("read templates failed: %w", err)
	}

	buildDir := filepath.Dir(buildFile)

	var filteredTmpls []*Template
	var skippedNoTest int
	for _, t := range tmpls {
		if filter != nil && !filter(t) {
			continue
		}
		if t.Test == nil {
			log.Printf("Template %s has no test configuration, skipping", t.Name)
			skippedNoTest++
			continue
		}
		if rayVersion != "" {
			hasBuildID := t.ClusterEnv != nil &&
				strings.TrimSpace(t.ClusterEnv.BuildID) != ""
			hasBYOD := t.ClusterEnv != nil && t.ClusterEnv.BYOD != nil &&
				strings.TrimSpace(t.ClusterEnv.BYOD.DockerImage) != ""
			if !hasBuildID && !hasBYOD {
				log.Printf(
					"Template %s has no build_id or docker_image, "+
						"skipping ray version override",
					t.Name,
				)
				continue
			}
			env, err := overrideClusterEnvRayVersion(t.ClusterEnv, rayVersion)
			if err != nil {
				return fmt.Errorf("override ray version for %q: %w", t.Name, err)
			}
			t.ClusterEnv = env
		}
		filteredTmpls = append(filteredTmpls, t)
	}
	if len(filteredTmpls) == 0 {
		if skippedNoTest > 0 {
			return fmt.Errorf("no templates with test configuration to run")
		}
		return fmt.Errorf("no templates to test")
	}

	var failed []string
	for _, t := range filteredTmpls {
		c := newWorkspaceTestConfig(t, cli, api, buildDir, false)

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

// setupEmptyWorkspace creates an empty workspace, starts it, and pushes
// template files into it.
func (c *WorkspaceTestConfig) setupEmptyWorkspace() {
	if awsConfigPath, ok := c.template.ComputeConfig["AWS"]; ok {
		c.computeConfig = generateComputeConfigName(awsConfigPath)
		resolvedPath := filepath.Join(c.buildDir, awsConfigPath)
		if err := c.anyscaleCLI.CreateComputeConfig(c.computeConfig, resolvedPath); err != nil {
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
		c.errs = append(c.errs, fmt.Errorf("push template to workspace failed: %w", err))
		return
	}

	unzipCmd := fmt.Sprintf("unzip -o %s.zip", c.tmplName)
	if err := c.anyscaleCLI.runCmdInWorkspace(c.workspaceName, unzipCmd); err != nil {
		c.errs = append(c.errs, fmt.Errorf("unzip template failed: %w", err))
		return
	}
}

// setupTemplateWorkspace launches a workspace from a template via the
// Anyscale API.
func (c *WorkspaceTestConfig) setupTemplateWorkspace() {
	cloudInfo, err := c.anyscaleCLI.GetDefaultCloud()
	if err != nil {
		c.errs = append(c.errs, fmt.Errorf("get default cloud failed: %w", err))
		return
	}

	projectInfo, err := c.anyscaleCLI.getDefaultProject(cloudInfo.ID)
	if err != nil {
		c.errs = append(c.errs, fmt.Errorf("get default project failed: %w", err))
		return
	}

	result, err := c.anyscaleAPI.launchTemplateInWorkspace(
		cloudInfo.ID, projectInfo.ID, c.tmplName, c.workspaceName,
	)
	if err != nil {
		c.errs = append(c.errs, fmt.Errorf("launch template in workspace failed: %w", err))
		return
	}

	workspaceName, okName := result["name"].(string)
	workspaceID, okID := result["id"].(string)
	if !okName || !okID {
		c.errs = append(c.errs, fmt.Errorf("unexpected response format: missing name or id"))
		return
	}
	c.workspaceName = workspaceName
	c.workspaceID = workspaceID
	if _, err := c.anyscaleCLI.waitForWorkspaceState(c.workspaceName, StateRunning); err != nil {
		c.errs = append(c.errs, fmt.Errorf("wait for workspace running state failed: %w", err))
		return
	}
}

func (c *WorkspaceTestConfig) cleanup() {
	log.Println("Cleaning up workspace...")
	if err := c.anyscaleCLI.terminateWorkspace(c.workspaceName); err != nil {
		c.errs = append(c.errs, fmt.Errorf("terminate workspace failed: %w", err))
		return
	}
	if _, err := c.anyscaleCLI.waitForWorkspaceState(c.workspaceName, StateTerminated); err != nil {
		c.errs = append(c.errs, fmt.Errorf("wait for workspace terminated state failed: %w", err))
		return
	}
	log.Println("Terminated workspace:", c.workspaceID)

	if err := c.anyscaleAPI.deleteWorkspaceByID(c.workspaceID); err != nil {
		c.errs = append(c.errs, fmt.Errorf("delete workspace failed: %w", err))
		return
	}
	log.Println("Deleted workspace:", c.workspaceID)
}

// Run sets up a workspace using the configured launcher, runs tests, and
// cleans up. It sets c.errs and c.success.
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

	if c.probe {
		c.setupTemplateWorkspace()
	} else {
		c.setupEmptyWorkspace()
	}

	defer c.cleanup()

	if len(c.errs) > 0 {
		return
	}

	// If tests_path is provided, zip and push test folder.
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
		if err := c.anyscaleCLI.runCmdInWorkspace(c.workspaceName, "unzip -o tests.zip"); err != nil {
			c.errs = append(c.errs, fmt.Errorf("unzip tests in workspace failed: %w", err))
			return
		}
	}

	// Run test command from test configuration.
	// Escape single quotes to prevent command injection via bash -c '...'.
	escapedCmd := strings.ReplaceAll(c.template.Test.Command, "'", "'\\''")
	testCommand := fmt.Sprintf("timeout %d bash -c '%s'", c.template.Test.TimeoutInSec, escapedCmd)
	if err := c.anyscaleCLI.runCmdInWorkspace(c.workspaceName, testCommand); err != nil {
		c.errs = append(c.errs, fmt.Errorf("run test command failed: %w", err))
		return
	}
}
