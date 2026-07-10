package raycicmd

import (
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
)

func TestAWSChainConfig(t *testing.T) {
	for _, test := range []struct {
		name  string
		roles []string
		want  string
	}{{
		name:  "single role",
		roles: []string{"arn:aws:iam::111111111111:role/r1"},
		want: strings.Join([]string{
			"[default]",
			"role_arn = arn:aws:iam::111111111111:role/r1",
			"credential_source = Ec2InstanceMetadata",
			"",
		}, "\n"),
	}, {
		name: "two roles",
		roles: []string{
			"arn:aws:iam::111111111111:role/r1",
			"arn:aws:iam::222222222222:role/r2",
		},
		want: strings.Join([]string{
			"[profile chain_0]",
			"role_arn = arn:aws:iam::111111111111:role/r1",
			"credential_source = Ec2InstanceMetadata",
			"",
			"[default]",
			"role_arn = arn:aws:iam::222222222222:role/r2",
			"source_profile = chain_0",
			"",
		}, "\n"),
	}, {
		name: "three roles",
		roles: []string{
			"arn:aws:iam::111111111111:role/r1",
			"arn:aws:iam::222222222222:role/r2",
			"arn:aws:iam::333333333333:role/r3",
		},
		want: strings.Join([]string{
			"[profile chain_0]",
			"role_arn = arn:aws:iam::111111111111:role/r1",
			"credential_source = Ec2InstanceMetadata",
			"",
			"[profile chain_1]",
			"role_arn = arn:aws:iam::222222222222:role/r2",
			"source_profile = chain_0",
			"",
			"[default]",
			"role_arn = arn:aws:iam::333333333333:role/r3",
			"source_profile = chain_1",
			"",
		}, "\n"),
	}} {
		t.Run(test.name, func(t *testing.T) {
			got := awsChainConfig(test.roles)
			if got != test.want {
				t.Errorf(
					"awsChainConfig(%v) = %q, want %q",
					test.roles, got, test.want,
				)
			}
		})
	}
}

// TestAWSChainSetupCommand_noBareDollar ensures every $ in the setup command
// is escaped as $$: rayci -upload pipes the pipeline through
// "buildkite-agent pipeline upload", which interpolates bare $VAR references
// against the uploader's environment (where these vars are unset), silently
// emptying the command.
func TestAWSChainSetupCommand_noBareDollar(t *testing.T) {
	rest := strings.ReplaceAll(awsChainSetupCommand, "$$", "")
	if strings.Contains(rest, "$") {
		t.Errorf(
			"awsChainSetupCommand has a bare $, which buildkite-agent"+
				" interpolates away at upload: %q",
			awsChainSetupCommand,
		)
	}
}

// TestAWSChainSetupCommand executes the prepended setup command with a real
// shell, the way the docker plugin runs step commands after buildkite-agent
// unescapes $$ to $ at upload, verifying that it materializes the config
// byte-for-byte and clears static credentials that would otherwise shadow
// the config-file chain.
func TestAWSChainSetupCommand(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "aws.config")
	leakFile := filepath.Join(dir, "leak")

	content := awsChainConfig([]string{
		"arn:aws:iam::111111111111:role/r1",
		"arn:aws:iam::222222222222:role/r2",
	})

	uploaded := strings.ReplaceAll(awsChainSetupCommand, "$$", "$")
	script := uploaded + `; printf '%s\n'` +
		` "${AWS_ACCESS_KEY_ID:-cleared}"` +
		` "${AWS_EC2_METADATA_DISABLED:-cleared}"` +
		` "${AWS_SHARED_CREDENTIALS_FILE:-unset}"` +
		` > "` + leakFile + `"`
	cmd := exec.Command("/bin/bash", "-ec", script)
	cmd.Env = append(os.Environ(),
		"RAYCI_AWS_CONFIG_B64="+
			base64.StdEncoding.EncodeToString([]byte(content)),
		"AWS_CONFIG_FILE="+configFile,
		"AWS_ACCESS_KEY_ID=AKIAFAKESTATICKEY",
		"AWS_EC2_METADATA_DISABLED=true",
		"AWS_SHARED_CREDENTIALS_FILE=/etc/fake-creds",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run setup command: %v, output: %s", err, out)
	}

	got, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	if string(got) != content {
		t.Errorf("config file = %q, want %q", got, content)
	}

	leak, err := os.ReadFile(leakFile)
	if err != nil {
		t.Fatalf("read leak file: %v", err)
	}
	// AWS_SHARED_CREDENTIALS_FILE is redirected to /dev/null rather than
	// unset: unsetting it would re-enable the default ~/.aws/credentials
	// lookup, where baked-in static keys shadow the config-file chain.
	want := "cleared\ncleared\n/dev/null\n"
	if string(leak) != want {
		t.Errorf("env after setup = %q, want %q", leak, want)
	}
}

// TestAWSChainConfig_parsedBySDK verifies the generated file against the
// real consumer: the AWS SDK shared-config loader that containers use to
// walk the chain.
func TestAWSChainConfig_parsedBySDK(t *testing.T) {
	roles := []string{
		"arn:aws:iam::111111111111:role/r1",
		"arn:aws:iam::222222222222:role/r2",
		"arn:aws:iam::333333333333:role/r3",
	}

	file := filepath.Join(t.TempDir(), "aws.config")
	content := awsChainConfig(roles)
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	ctx := context.Background()
	load := func(profile string) awsconfig.SharedConfig {
		c, err := awsconfig.LoadSharedConfigProfile(
			ctx, profile,
			func(o *awsconfig.LoadSharedConfigOptions) {
				o.ConfigFiles = []string{file}
				o.CredentialsFiles = []string{}
			},
		)
		if err != nil {
			t.Fatalf("load profile %q: %v", profile, err)
		}
		return c
	}

	root := load("chain_0")
	if root.RoleARN != roles[0] {
		t.Errorf("chain_0 role_arn = %q, want %q", root.RoleARN, roles[0])
	}
	if root.CredentialSource != "Ec2InstanceMetadata" {
		t.Errorf(
			"chain_0 credential_source = %q, want Ec2InstanceMetadata",
			root.CredentialSource,
		)
	}

	mid := load("chain_1")
	if mid.RoleARN != roles[1] {
		t.Errorf("chain_1 role_arn = %q, want %q", mid.RoleARN, roles[1])
	}
	if mid.SourceProfileName != "chain_0" {
		t.Errorf(
			"chain_1 source_profile = %q, want chain_0",
			mid.SourceProfileName,
		)
	}

	dst := load("default")
	if dst.RoleARN != roles[2] {
		t.Errorf("default role_arn = %q, want %q", dst.RoleARN, roles[2])
	}
	if dst.SourceProfileName != "chain_1" {
		t.Errorf(
			"default source_profile = %q, want chain_1",
			dst.SourceProfileName,
		)
	}
}
