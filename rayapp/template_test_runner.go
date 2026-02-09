package rayapp

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
	success       bool
	errs          []error
}
