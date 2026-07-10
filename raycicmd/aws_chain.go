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
	// AWS_CONFIG_FILE, where AWS SDKs pick it up. The unset list covers
	// every credential or IMDS override that ranks above the shared-config
	// file in SDK default chains: the docker plugin runs commands under a
	// login shell, and image profile scripts exporting any of these would
	// silently shadow or break the chain. AWS_SHARED_CREDENTIALS_FILE is
	// redirected to /dev/null rather than unset — unsetting it would
	// re-enable the default ~/.aws/credentials lookup, where baked-in
	// static keys shadow the chain. $ is escaped as $$ because
	// "buildkite-agent pipeline upload" interpolates bare $VAR references
	// against the uploader's environment.
	awsChainSetupCommand = `unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY` +
		` AWS_SESSION_TOKEN AWS_ACCESS_KEY AWS_SECRET_KEY AWS_SECURITY_TOKEN` +
		` AWS_PROFILE AWS_DEFAULT_PROFILE` +
		` AWS_WEB_IDENTITY_TOKEN_FILE` +
		` AWS_ROLE_ARN AWS_CONTAINER_CREDENTIALS_RELATIVE_URI` +
		` AWS_CONTAINER_CREDENTIALS_FULL_URI` +
		` AWS_EC2_METADATA_DISABLED AWS_EC2_METADATA_SERVICE_ENDPOINT` +
		` AWS_ENDPOINT_URL AWS_ENDPOINT_URL_STS` +
		` && export AWS_SHARED_CREDENTIALS_FILE=/dev/null` +
		` && printf '%s' "$$RAYCI_AWS_CONFIG_B64"` +
		` | base64 -d > "$$AWS_CONFIG_FILE"`
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

// checkAWSRoleARN rejects values that cannot work as a literal role ARN.
// The value is rendered verbatim into the generated INI and base64-encoded
// at generation time: whitespace would inject arbitrary config lines, and
// Buildkite matrix tokens or $ env references are substituted only after
// generation, where they cannot reach inside the base64 blob.
func checkAWSRoleARN(r string) error {
	if !strings.HasPrefix(r, "arn:") {
		return fmt.Errorf("role %q is not an ARN", r)
	}
	if strings.ContainsAny(r, " \t\n\r\"'$") || strings.Contains(r, "{{") {
		return fmt.Errorf("role %q contains unsupported characters", r)
	}
	return nil
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
		if i == 0 {
			fmt.Fprintln(b, "credential_source = Ec2InstanceMetadata")
		} else {
			fmt.Fprintf(b, "source_profile = chain_%d\n", i-1)
		}
	}
	return b.String()
}
