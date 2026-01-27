package raycicmd

const (
	dockerPlugin        = "docker#v5.8.0"
	awsAssumeRolePlugin = "cultureamp/aws-assume-role#v0.2.0"

	macosSandboxPlugin = "ray-project/macos-sandbox#v1.0.7"
	macosJobEnv        = "MACOS"
	macosDenyFileRead  = "/usr/local/etc/buildkite-agent/buildkite-agent.cfg"

	windowsJobEnv = "WINDOWS"
)

// pluginBuildParams contains the parameters needed to build the plugins slice.
type pluginBuildParams struct {
	jobEnv                string
	jobEnvImage           string
	assumeRole            string
	assumeRoleDuration    int
	assumeRoleDurationSet bool
	dockerPluginConfig    *stepDockerPluginConfig
}

// pluginBuildResult contains the result of building plugins.
type pluginBuildResult struct {
	plugins       []any
	artifactPaths []string
}

// buildPlugins builds the plugins slice and artifact paths based on the job environment.
func buildPlugins(p *pluginBuildParams) *pluginBuildResult {
	switch p.jobEnv {
	case windowsJobEnv: // a special job env for windows
		return &pluginBuildResult{
			plugins: []any{map[string]any{
				dockerPlugin: makeRayWindowsDockerPlugin(p.dockerPluginConfig),
			}},
			artifactPaths: windowsArtifactPaths,
		}
	case macosJobEnv: // a special job env for macos
		return &pluginBuildResult{
			plugins: []any{map[string]any{
				macosSandboxPlugin: map[string]string{
					"deny-file-read": macosDenyFileRead,
				},
			}},
			artifactPaths: nil,
		}
	default:
		// default Linux Job env.
		var plugins []any
		if p.assumeRole != "" {
			duration := p.assumeRoleDuration
			if !p.assumeRoleDurationSet {
				duration = 900 // min value to assume role
			}
			plugins = append(plugins, map[string]any{
				awsAssumeRolePlugin: map[string]any{
					"role":     p.assumeRole,
					"duration": duration,
				},
			})
		}

		plugins = append(plugins, map[string]any{
			dockerPlugin: makeRayDockerPlugin(p.jobEnvImage, p.dockerPluginConfig),
		})

		return &pluginBuildResult{
			plugins:       plugins,
			artifactPaths: defaultArtifactPaths,
		}
	}
}
