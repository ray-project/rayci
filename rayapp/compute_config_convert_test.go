package rayapp

import (
	"os/exec"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

func mustParseYAML(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := yaml.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("parse expected yaml: %v", err)
	}
	return m
}

func TestConvertNewComputeConfigToLegacy(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "head plus worker pins head unschedulable",
			input: `
head_node:
  instance_type: m5.2xlarge
worker_nodes:
- name: cpu_worker
  instance_type: m5.2xlarge
  max_nodes: 2
`,
			want: `
head_node_type:
  name: head-node
  instance_type: m5.2xlarge
  resources: {CPU: 0, GPU: 0}
worker_node_types:
- name: cpu_worker
  instance_type: m5.2xlarge
  min_workers: 0
  max_workers: 2
  use_spot: false
  fallback_to_ondemand: false
auto_select_worker_config: false
flags: {allow-cross-zone-autoscaling: false}
`,
		},
		{
			name: "explicit head resources preserved; PREFER_SPOT worker",
			input: `
head_node:
  instance_type: m5.2xlarge
  resources:
    CPU: 0
worker_nodes:
- name: gpu-worker
  instance_type: g6.2xlarge
  max_nodes: 4
  market_type: PREFER_SPOT
`,
			want: `
head_node_type:
  name: head-node
  instance_type: m5.2xlarge
  resources: {CPU: 0}
worker_node_types:
- name: gpu-worker
  instance_type: g6.2xlarge
  min_workers: 0
  max_workers: 4
  use_spot: true
  fallback_to_ondemand: true
auto_select_worker_config: false
flags: {allow-cross-zone-autoscaling: false}
`,
		},
		{
			name: "auto-select with cross-zone, no workers",
			input: `
head_node:
  instance_type: m5.2xlarge
  resources:
    CPU: 0
enable_cross_zone_scaling: true
auto_select_worker_config: true
`,
			want: `
head_node_type:
  name: head-node
  instance_type: m5.2xlarge
  resources: {CPU: 0}
worker_node_types: []
auto_select_worker_config: true
flags: {allow-cross-zone-autoscaling: true}
`,
		},
		{
			name: "advanced config plus auto-select pins head unschedulable",
			input: `
head_node:
  instance_type: n2-standard-8
advanced_instance_config:
  instance_properties:
    disks:
    - boot: true
auto_select_worker_config: true
`,
			want: `
head_node_type:
  name: head-node
  instance_type: n2-standard-8
  resources: {CPU: 0, GPU: 0}
worker_node_types: []
auto_select_worker_config: true
flags: {allow-cross-zone-autoscaling: false}
advanced_configurations_json:
  instance_properties:
    disks:
    - boot: true
`,
		},
		{
			name: "head-only schedulable",
			input: `
head_node:
  instance_type: m5.2xlarge
worker_nodes: []
`,
			want: `
head_node_type:
  name: head-node
  instance_type: m5.2xlarge
worker_node_types: []
auto_select_worker_config: false
flags: {allow-cross-zone-autoscaling: false}
`,
		},
		{
			name: "declarative nodes: memory quantity strings become bytes",
			input: `
head_node:
  required_resources:
    CPU: 4
    memory: 8Gi
  resources:
    CPU: 0
worker_nodes:
- name: gpu_worker
  required_resources:
    CPU: 4
    memory: 16Gi
    GPU: 1
  required_labels:
    ray.io/accelerator-type: T4
  min_nodes: 2
  max_nodes: 2
`,
			want: `
head_node_type:
  name: head-node
  required_resources: {CPU: 4, memory: 8589934592}
  resources: {CPU: 0}
worker_node_types:
- name: gpu_worker
  required_resources: {CPU: 4, memory: 17179869184, GPU: 1}
  required_labels: {ray.io/accelerator-type: T4}
  min_workers: 2
  max_workers: 2
  use_spot: false
  fallback_to_ondemand: false
auto_select_worker_config: false
flags: {allow-cross-zone-autoscaling: false}
`,
		},
		{
			name: "declarative nodes: integer bytes pass through unchanged",
			input: `
head_node:
  required_resources:
    CPU: 4
    memory: 8589934592
worker_nodes: []
`,
			want: `
head_node_type:
  name: head-node
  required_resources: {CPU: 4, memory: 8589934592}
worker_node_types: []
auto_select_worker_config: false
flags: {allow-cross-zone-autoscaling: false}
`,
		},
		{
			name: "zones, per-node advanced, labels, default worker name/counts, SPOT",
			input: `
head_node:
  instance_type: m5.2xlarge
  advanced_instance_config: {head: cfg}
worker_nodes:
- instance_type: g5.xlarge
  market_type: SPOT
  labels: {team: ml}
zones: [us-west-2a, us-west-2b]
`,
			want: `
head_node_type:
  name: head-node
  instance_type: m5.2xlarge
  resources: {CPU: 0, GPU: 0}
  advanced_configurations_json: {head: cfg}
worker_node_types:
- name: g5.xlarge
  instance_type: g5.xlarge
  labels: {team: ml}
  min_workers: 0
  max_workers: 10
  use_spot: true
  fallback_to_ondemand: false
auto_select_worker_config: false
flags: {allow-cross-zone-autoscaling: false}
allowed_azs: [us-west-2a, us-west-2b]
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := convertNewComputeConfigToLegacy([]byte(tt.input))
			if err != nil {
				t.Fatalf("convert: %v", err)
			}
			var got map[string]any
			if err := yaml.Unmarshal(out, &got); err != nil {
				t.Fatalf("parse output: %v", err)
			}
			want := mustParseYAML(t, tt.want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("mismatch\n got:  %#v\n want: %#v", got, want)
			}
		})
	}
}

func TestConvertNewComputeConfigToLegacyErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unknown top-level key", "head_node: {instance_type: m5.2xlarge}\nbogus_top: 1\n"},
		{"unknown node key", "head_node: {instance_type: m5.2xlarge, bogus_node: 1}\n"},
		{
			"bad market_type",
			"head_node: {instance_type: m5.2xlarge}\nworker_nodes:\n- {instance_type: x, market_type: BOGUS}\n",
		},
		{"missing head_node", "worker_nodes: []\n"},
		{
			"non-integer min_nodes",
			"head_node: {instance_type: m5.2xlarge}\nworker_nodes:\n- {instance_type: x, min_nodes: five}\n",
		},
		{
			"auto_select not a bool",
			"head_node: {instance_type: m5.2xlarge}\nauto_select_worker_config: \"yes\"\n",
		},
		{"zones not a list", "head_node: {instance_type: m5.2xlarge}\nzones: us-west-2a\n"},
		{
			"max_nodes < min_nodes",
			"head_node: {instance_type: m5.2xlarge}\nworker_nodes:\n- {instance_type: x, min_nodes: 5, max_nodes: 2}\n",
		},
		{"head_node not a mapping", "head_node: just-a-string\n"},
		{"flags not a mapping", "head_node: {instance_type: m5.2xlarge}\nflags: not-a-map\n"},
		{
			"worker name not a string",
			"head_node: {instance_type: m5.2xlarge}\nworker_nodes:\n- {name: 123, instance_type: x}\n",
		},
		{
			"worker instance_type not a string",
			"head_node: {instance_type: m5.2xlarge}\nworker_nodes:\n- {instance_type: 123}\n",
		},
		{
			"worker market_type not a string",
			"head_node: {instance_type: m5.2xlarge}\nworker_nodes:\n- {instance_type: x, market_type: true}\n",
		},
		{
			"negative min_nodes",
			"head_node: {instance_type: m5.2xlarge}\nworker_nodes:\n- {instance_type: x, min_nodes: -1}\n",
		},
		{
			"negative max_nodes",
			"head_node: {instance_type: m5.2xlarge}\nworker_nodes:\n- {instance_type: x, max_nodes: -1}\n",
		},
		{
			"fractional min_nodes",
			"head_node: {instance_type: m5.2xlarge}\nworker_nodes:\n- {instance_type: x, min_nodes: 1.5}\n",
		},
		{"head flags not a mapping", "head_node: {instance_type: m5.2xlarge, flags: [a, b]}\n"},
		{
			"worker flags not a mapping",
			"head_node: {instance_type: m5.2xlarge}\nworker_nodes:\n- {instance_type: x, flags: not-a-map}\n",
		},
		{"bad head memory quantity", "head_node: {required_resources: {CPU: 4, memory: 8GB}}\n"},
		{
			"bad worker memory quantity",
			"head_node: {instance_type: m5.2xlarge}\nworker_nodes:\n- {instance_type: x, required_resources: {memory: 100m}}\n",
		},
		{"required_resources not a mapping", "head_node: {required_resources: 8Gi}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := convertNewComputeConfigToLegacy([]byte(tt.input)); err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}

// memoryQuantityGoldens maps a quantity string to its value in bytes. Every
// entry is cross-checked against the SDK's _parse_memory_string by
// TestParseMemoryBytesMatchesSDK, which is what keeps this Go port from
// drifting from the Python one it mirrors.
var memoryQuantityGoldens = map[string]int64{
	// The shapes templates actually use.
	"8Gi":   8589934592,
	"16Gi":  17179869184,
	"32Gi":  34359738368,
	"4Gi":   4294967296,
	"512Mi": 536870912,
	// Every accepted suffix, so a wrong multiplier cannot hide.
	"1Ki": 1024,
	"1Mi": 1048576,
	"1Gi": 1073741824,
	"1Ti": 1099511627776,
	"1Pi": 1125899906842624,
	"1Ei": 1152921504606846976,
	"1k":  1000,
	"1M":  1000000,
	"1G":  1000000000,
	"1T":  1000000000000,
	"1P":  1000000000000000,
	"1E":  1000000000000000000,
	// No suffix is already bytes.
	"1000": 1000,
	"0":    0,
	"0Gi":  0,
	// A fraction is fine as long as it lands on a whole number of bytes.
	"1.5Gi":  1610612736,
	"2.25Gi": 2415919104,
	"0.5Mi":  524288,
	// Exponent notation is part of the quantity grammar.
	"1e3": 1000,
}

func TestParseMemoryBytes(t *testing.T) {
	for _, q := range sortedKeys(memoryQuantityGoldens) {
		t.Run(q, func(t *testing.T) {
			got, err := parseMemoryBytes(q)
			if err != nil {
				t.Fatalf("parseMemoryBytes(%q): %v", q, err)
			}
			if want := memoryQuantityGoldens[q]; got != want {
				t.Errorf("parseMemoryBytes(%q) = %d, want %d", q, got, want)
			}
		})
	}

	// Numbers are already bytes and pass through.
	nums := []struct {
		name string
		in   any
		want int64
	}{
		{"int", 8589934592, 8589934592},
		{"int64", int64(1024), 1024},
		{"float64 whole", float64(1024), 1024},
		{"zero", 0, 0},
	}
	for _, tt := range nums {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMemoryBytes(tt.in)
			if err != nil {
				t.Fatalf("parseMemoryBytes(%v): %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("parseMemoryBytes(%v) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// TestParseMemoryBytesErrors covers what we reject. The first group is what
// the SDK rejects too; the second is forms the SDK would convert silently into
// something the author did not mean, which is why this port is stricter.
func TestParseMemoryBytesErrors(t *testing.T) {
	tests := []struct {
		name string
		in   any
	}{
		{"lowercase suffix", "8gi"},
		{"GB is not a k8s suffix", "8GB"},
		{"uppercase K is not a k8s suffix", "1K"},
		{"empty", ""},
		{"not a number", "abc"},
		{"suffix only", "Gi"},
		{"bare decimal suffix", "k"},
		{"just a dot", "."},
		{"just a sign", "+"},
		{"two dots", "1.2.3"},
		{"trailing junk", "8Gi "},
		{"space before suffix", "1 Gi"},
		{"bool", true},
		{"list", []any{1}},
		{"mapping", map[string]any{"a": 1}},
		{"fractional float", 1.5},
		{"negative int", -1},
		{"negative int64", int64(-1)},
		{"negative float", -1.0},
		{"negative quantity", "-1"},
		{"saturates int64", "16Ei"},

		// Parse fine as quantities, but are not a whole number of bytes, so
		// they could not reach the backend's integer memory field intact:
		{"milli is 100 millibytes", "100m"},
		{"fraction of a byte", "1.5"},
		{"fractional mebibytes", "3.7Mi"},
		{"fractional kibibytes", "1.1Ki"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := parseMemoryBytes(tt.in); err == nil {
				t.Errorf("parseMemoryBytes(%v) = %d, want an error", tt.in, got)
			}
		})
	}
}

// sdkParseMemoryScript prints one parsed value per input line, and exits 97
// when the anyscale SDK is not importable so the caller can skip.
const sdkParseMemoryScript = `
import sys
try:
    from anyscale.compute_config.models import _parse_memory_string
except Exception:
    sys.exit(97)
for line in sys.stdin.read().splitlines():
    print(_parse_memory_string(line))
`

// TestParseMemoryBytesMatchesSDK checks every golden against the SDK function
// this port mirrors, so the two cannot drift apart unnoticed. Skipped where
// python3 or the anyscale package is unavailable.
func TestParseMemoryBytesMatchesSDK(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not available")
	}

	quantities := sortedKeys(memoryQuantityGoldens)
	cmd := exec.Command(python, "-c", sdkParseMemoryScript)
	cmd.Stdin = strings.NewReader(strings.Join(quantities, "\n"))
	out, err := cmd.Output()
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 97 {
		t.Skip("anyscale SDK not importable")
	}
	if err != nil {
		t.Fatalf("run SDK parser: %v", err)
	}

	lines := strings.Fields(string(out))
	if len(lines) != len(quantities) {
		t.Fatalf("got %d results for %d quantities: %q", len(lines), len(quantities), out)
	}
	for i, q := range quantities {
		want, err := strconv.ParseInt(lines[i], 10, 64)
		if err != nil {
			t.Fatalf("parse SDK result %q for %q: %v", lines[i], q, err)
		}
		if golden := memoryQuantityGoldens[q]; golden != want {
			t.Errorf("golden for %q is %d, but the SDK says %d", q, golden, want)
		}
		got, err := parseMemoryBytes(q)
		if err != nil {
			t.Fatalf("parseMemoryBytes(%q): %v", q, err)
		}
		if got != want {
			t.Errorf("parseMemoryBytes(%q) = %d, but the SDK says %d", q, got, want)
		}
	}
}

func sortedKeys(m map[string]int64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
