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
	// RAYCI_AWS_CONFIG_B64 container env var, so multi-line content cannot
	// be mangled on its way through docker env propagation) to
	// awsConfigFilePath, where AWS SDKs pick it up. The leading test -n
	// fails the step outright if the variable arrives empty — decoding an
	// empty value would write an empty config and the SDKs would silently
	// fall through to raw instance-profile credentials. The unset list
	// covers the credential providers that outrank the shared-config file
	// in SDK default chains (static env keys, profile selectors,
	// web-identity, container/pod credentials) plus
	// AWS_EC2_METADATA_DISABLED, which would dead-end the chain's IMDS
	// root. The docker plugin runs commands under a login
	// shell, so image profile scripts could export any of these before the
	// step's commands run. For the same reason AWS_CONFIG_FILE is
	// force-exported to the literal path, and AWS_SHARED_CREDENTIALS_FILE
	// is redirected to /dev/null rather than unset — unsetting it would
	// re-enable the default ~/.aws/credentials lookup. $ is escaped as $$
	// because "buildkite-agent pipeline upload" interpolates bare $VAR
	// references against the uploader's environment.
	awsChainSetupCommand = `test -n "$$RAYCI_AWS_CONFIG_B64"` +
		` && unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY` +
		` AWS_SESSION_TOKEN` +
		` AWS_PROFILE AWS_DEFAULT_PROFILE` +
		` AWS_ROLE_ARN AWS_WEB_IDENTITY_TOKEN_FILE` +
		` AWS_CONTAINER_CREDENTIALS_RELATIVE_URI` +
		` AWS_CONTAINER_CREDENTIALS_FULL_URI` +
		` AWS_CONTAINER_CREDENTIALS_TOKEN_FILE` +
		` AWS_CONTAINER_AUTHORIZATION_TOKEN` +
		` AWS_EC2_METADATA_DISABLED` +
		` && export AWS_SHARED_CREDENTIALS_FILE=/dev/null` +
		` AWS_CONFIG_FILE=` + awsConfigFilePath +
		` && printf '%s' "$$RAYCI_AWS_CONFIG_B64"` +
		` | base64 -d > ` + awsConfigFilePath
)

// stepAWSAssumeRoles reads the aws_assume_role step key: a single role ARN
// or a list of ARNs in assume order. Returns nil when the key is absent.
func stepAWSAssumeRoles(step map[string]any) ([]string, error) {
	v, ok := step["aws_assume_role"]
	if !ok {
		return nil, nil
	}

	roles, err := scalarStrings(v)
	if err != nil {
		return nil, fmt.Errorf("aws_assume_role: %w", err)
	}
	if len(roles) == 0 {
		return nil, fmt.Errorf("aws_assume_role is empty")
	}
	for _, r := range roles {
		if err := checkAWSRoleARN(r); err != nil {
			return nil, fmt.Errorf("aws_assume_role: %w", err)
		}
	}
	return roles, nil
}

// checkAWSRoleARN rejects values that cannot work as a literal IAM role
// ARN of the shape arn:<partition>:iam::<12-digit account>:role/<name>.
// The value is rendered verbatim into the generated INI and base64-encoded
// at generation time, so only ARN-legal characters pass: whitespace or
// INI-special bytes would inject or truncate config lines, and Buildkite
// matrix tokens or $ env references are substituted only after generation,
// where they cannot reach inside the base64 blob.
func checkAWSRoleARN(r string) error {
	parts := strings.SplitN(r, ":", 6)
	if len(parts) != 6 || parts[0] != "arn" ||
		!isARNPartition(parts[1]) ||
		parts[2] != "iam" || parts[3] != "" ||
		len(parts[4]) != 12 ||
		strings.Trim(parts[4], "0123456789") != "" ||
		!strings.HasPrefix(parts[5], "role/") ||
		parts[5] == "role/" {
		return fmt.Errorf("role %q is not an IAM role ARN", r)
	}
	// SplitN leaves any extra colons in the resource field, so checking
	// the role path/name bytes also rejects colon-suffixed values.
	name := strings.TrimPrefix(parts[5], "role/")
	for i := 0; i < len(name); i++ {
		if c := name[i]; !isSTSNameByte(c) && c != '/' {
			return fmt.Errorf("role %q contains unsupported characters", r)
		}
	}
	return nil
}

// isARNPartition reports whether s looks like an ARN partition
// (aws, aws-cn, aws-us-gov): lowercase letters and hyphens.
func isARNPartition(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; !(c >= 'a' && c <= 'z' || c == '-') {
			return false
		}
	}
	return true
}

// isSTSNameByte reports whether c is in the STS name charset [\w+=,.@-],
// shared by role session names and the non-separator bytes of role ARNs.
func isSTSNameByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
		c == '+', c == '=', c == ',', c == '.',
		c == '@', c == '_', c == '-':
		return true
	}
	return false
}

// awsSessionName derives a valid STS role session name from the build ID:
// STS restricts session names to 64 chars of [\w+=,.@-], and the name is
// rendered verbatim into the generated INI. The build ID comes from
// infrastructure rather than the step author, so invalid bytes map to "-"
// instead of failing generation.
func awsSessionName(buildID string) string {
	const prefix = "rayci-"
	b := []byte(buildID)
	for i, c := range b {
		if !isSTSNameByte(c) {
			b[i] = '-'
		}
	}
	name := prefix + string(b)
	if len(name) > 64 {
		name = name[:64]
	}
	return name
}

// awsChainConfig generates an AWS shared-config (INI) file that expresses
// roles as an assume-role chain rooted at the EC2 instance profile. roles
// must be non-empty and listed in assume order: the first role is assumed
// from instance metadata (IMDS), each subsequent role from the previous one,
// and the last role is the [default] profile that SDKs run as. Because the
// chain roots at IMDS, SDKs re-derive the whole chain before expiry instead
// of holding frozen credentials. sessionName labels every hop's STS session
// so CloudTrail entries map back to the build.
func awsChainConfig(roles []string, sessionName string) string {
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
		fmt.Fprintf(b, "role_session_name = %s\n", sessionName)
		// Chained-hop sessions cap at one hour; requesting the cap keeps
		// credentials that consumers snapshot out of the SDK refresh loop
		// (exported to subprocesses or remote workers) alive as long as
		// STS allows.
		fmt.Fprintln(b, "duration_seconds = 3600")
		if i == 0 {
			fmt.Fprintln(b, "credential_source = Ec2InstanceMetadata")
		} else {
			fmt.Fprintf(b, "source_profile = chain_%d\n", i-1)
		}
	}
	return b.String()
}
