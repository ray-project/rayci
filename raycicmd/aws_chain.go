package raycicmd

import (
	"fmt"
	"strings"
)

const (
	// awsConfigFilePath is where awsChainSetupCommand materializes the
	// generated chain config inside the container. /tmp rather than the
	// workdir: the workdir is the source tree, and a file appearing there
	// can invalidate build caches or leak into artifacts.
	awsConfigFilePath = "/tmp/rayci-aws.config"

	// awsChainSetupCommand is prepended to a step's commands to write the
	// generated chain config (delivered base64-encoded via the
	// RAYCI_AWS_CONFIG_B64 step env var, so multi-line content cannot be
	// mangled on its way through Buildkite and docker env propagation) to
	// AWS_CONFIG_FILE, where AWS SDKs pick it up. The unset guards against
	// image profile scripts: the docker plugin runs commands under a login
	// shell, and any static AWS_* vars exported there would take precedence
	// over the config-file chain in every SDK.
	awsChainSetupCommand = `unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY` +
		` AWS_SESSION_TOKEN AWS_PROFILE AWS_DEFAULT_PROFILE` +
		` && printf '%s' "$RAYCI_AWS_CONFIG_B64" | base64 -d > "$AWS_CONFIG_FILE"`
)

// stepAWSAssumeRoles reads the aws_assume_role step key: a single role ARN
// or a list of ARNs in assume order. Returns nil when the key is absent.
func stepAWSAssumeRoles(step map[string]any) ([]string, error) {
	v, ok := step["aws_assume_role"]
	if !ok || v == nil {
		return nil, nil
	}
	roles := toStringList(v)
	if list, isList := v.([]any); isList && len(roles) != len(list) {
		return nil, fmt.Errorf("aws_assume_role has non-string entries: %v", v)
	}
	if len(roles) == 0 {
		return nil, fmt.Errorf("aws_assume_role is empty")
	}
	return roles, nil
}

// awsChainConfig generates an AWS shared-config (INI) file that expresses
// roles as an assume-role chain rooted at the EC2 instance profile. roles
// must be non-empty and listed in assume order: the first role is assumed
// from instance metadata (IMDS), each subsequent role from the previous one,
// and the last role is the [default] profile that SDKs run as. Because the
// chain roots at IMDS, SDKs re-derive the whole chain before expiry instead
// of holding frozen credentials.
func awsChainConfig(roles []string) string {
	profile := func(i int) string {
		if i == len(roles)-1 {
			return "default"
		}
		return fmt.Sprintf("profile chain_%d", i)
	}

	b := new(strings.Builder)
	for i, role := range roles {
		if i > 0 {
			fmt.Fprintln(b)
		}
		fmt.Fprintf(b, "[%s]\n", profile(i))
		fmt.Fprintf(b, "role_arn = %s\n", role)
		if i == 0 {
			fmt.Fprintln(b, "credential_source = Ec2InstanceMetadata")
		} else {
			fmt.Fprintf(b, "source_profile = chain_%d\n", i-1)
		}
	}
	return b.String()
}

// prependCommand inserts line before a step's existing command(s). The
// docker plugin runs all command lines in a single shell invocation, so
// files written and variables exported by line are visible to the rest.
func prependCommand(step map[string]any, line string) error {
	for _, key := range []string{"commands", "command"} {
		v, ok := step[key]
		if !ok || v == nil {
			continue
		}
		cmds := toStringList(v)
		if list, isList := v.([]any); isList && len(cmds) != len(list) {
			return fmt.Errorf("%s has non-string entries: %v", key, v)
		}
		if len(cmds) == 0 {
			continue
		}
		step[key] = append([]string{line}, cmds...)
		return nil
	}
	return fmt.Errorf("step has no command")
}
