package rayapp

import (
	"reflect"
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
		{"bad market_type", "head_node: {instance_type: m5.2xlarge}\nworker_nodes:\n- {instance_type: x, market_type: BOGUS}\n"},
		{"missing head_node", "worker_nodes: []\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := convertNewComputeConfigToLegacy([]byte(tt.input)); err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}
