package rayapp

import (
	"errors"
	"reflect"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

func TestDeriveK8SComputeConfig(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			// Reproduces the hand-written configs/getting-started/k8s.yaml.
			name: "unschedulable head gets coordinator sizing, worker gets the instance shape",
			input: `
head_node:
  instance_type: m5.2xlarge
  resources:
    CPU: 0
worker_nodes:
- name: cpu_worker
  instance_type: m5.2xlarge
  max_nodes: 2
`,
			want: `
head_node:
  required_resources: {CPU: 4, memory: 8589934592}
  resources: {CPU: 0}
worker_nodes:
- name: cpu_worker
  required_resources: {CPU: 8, memory: 34359738368}
  max_nodes: 2
`,
		},
		{
			// Reproduces configs/pytorch-fsdp/k8s.yaml, bar the group name,
			// which is carried over from the VM config rather than invented.
			name: "GPU worker carries the accelerator type as a label",
			input: `
head_node:
  instance_type: m5.2xlarge
  resources:
    CPU: 0
worker_nodes:
- name: gpu_worker
  instance_type: g4dn.xlarge
  min_nodes: 2
  max_nodes: 2
`,
			want: `
head_node:
  required_resources: {CPU: 4, memory: 8589934592}
  resources: {CPU: 0}
worker_nodes:
- name: gpu_worker
  required_resources: {CPU: 4, memory: 17179869184, GPU: 1}
  required_labels: {ray.io/accelerator-type: T4}
  min_nodes: 2
  max_nodes: 2
`,
		},
		{
			name: "schedulable head takes the full instance shape",
			input: `
head_node:
  instance_type: m5.4xlarge
worker_nodes:
- instance_type: g6.2xlarge
  max_nodes: 4
  market_type: SPOT
`,
			want: `
head_node:
  required_resources: {CPU: 16, memory: 68719476736}
worker_nodes:
- required_resources: {CPU: 8, memory: 34359738368, GPU: 1}
  required_labels: {ray.io/accelerator-type: L4}
  max_nodes: 4
  market_type: SPOT
`,
		},
		{
			// Reproduces the hand-written configs/basic-single-node/k8s.yaml.
			name: "auto-select stands in one group shaped like the head's machine",
			input: `
head_node:
  instance_type: m5.2xlarge
  resources:
    CPU: 0
auto_select_worker_config: true
`,
			want: `
head_node:
  required_resources: {CPU: 4, memory: 8589934592}
  resources: {CPU: 0}
worker_nodes:
- name: cpu_worker
  required_resources: {CPU: 8, memory: 34359738368}
  max_nodes: 1
`,
		},
		{
			name: "head-only cluster with a schedulable head needs no workers",
			input: `
head_node:
  instance_type: m5.2xlarge
`,
			want: `
head_node:
  required_resources: {CPU: 8, memory: 34359738368}
`,
		},
		{
			name: "a declarative worker is left alone too",
			input: `
head_node:
  instance_type: m5.2xlarge
  resources:
    CPU: 0
worker_nodes:
- name: already_declared
  required_resources:
    CPU: 3
    memory: 6Gi
  max_nodes: 4
- name: from_instance
  instance_type: g5.xlarge
  max_nodes: 1
`,
			want: `
head_node:
  required_resources: {CPU: 4, memory: 8589934592}
  resources: {CPU: 0}
worker_nodes:
- name: already_declared
  required_resources: {CPU: 3, memory: 6Gi}
  max_nodes: 4
- name: from_instance
  required_resources: {CPU: 4, memory: 17179869184, GPU: 1}
  required_labels: {ray.io/accelerator-type: A10G}
  max_nodes: 1
`,
		},
		{
			name: "a node that is already declarative is left alone",
			input: `
head_node:
  required_resources:
    CPU: 2
    memory: 4Gi
  resources:
    CPU: 0
worker_nodes:
- name: cpu_worker
  instance_type: m5.2xlarge
  max_nodes: 1
`,
			want: `
head_node:
  required_resources: {CPU: 2, memory: 4Gi}
  resources: {CPU: 0}
worker_nodes:
- name: cpu_worker
  required_resources: {CPU: 8, memory: 34359738368}
  max_nodes: 1
`,
		},
		{
			name: "an explicit accelerator label wins over the table",
			input: `
head_node:
  instance_type: m5.2xlarge
  resources:
    CPU: 0
worker_nodes:
- instance_type: g5.xlarge
  required_labels:
    ray.io/accelerator-type: A10
  max_nodes: 1
`,
			want: `
head_node:
  required_resources: {CPU: 4, memory: 8589934592}
  resources: {CPU: 0}
worker_nodes:
- required_resources: {CPU: 4, memory: 17179869184, GPU: 1}
  required_labels: {ray.io/accelerator-type: A10}
  max_nodes: 1
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := deriveK8SComputeConfig([]byte(tt.input))
			if err != nil {
				t.Fatalf("derive: %v", err)
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

// TestDeriveK8SComputeConfigSkips covers what cannot be translated. Every case
// reports errNoK8SDerivation, which the builder logs and moves past -- a
// template with no K8S config is no worse off than it was before.
func TestDeriveK8SComputeConfigSkips(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"auto-select with a declarative head has no shape to stand in",
			"head_node: {required_resources: {CPU: 2}}\nauto_select_worker_config: true\n"},
		{"legacy schema",
			"head_node_type: {name: head, instance_type: m5.2xlarge}\nworker_node_types: []\n"},
		{"instance type not in the table",
			"head_node: {instance_type: m5.2xlarge}\nworker_nodes:\n- {instance_type: p4d.24xlarge}\n"},
		{"no worker nodes to translate",
			"head_node: {instance_type: m5.2xlarge, resources: {CPU: 0}}\n"},
		{"no head node",
			"worker_nodes:\n- {instance_type: m5.2xlarge}\n"},
		{"a node setting both instance_type and required_resources",
			"head_node: {instance_type: m5.2xlarge, required_resources: {CPU: 2}}\nworker_nodes:\n- {instance_type: m5.2xlarge}\n"},
		{"worker without an instance type",
			"head_node: {instance_type: m5.2xlarge}\nworker_nodes:\n- {name: mystery, max_nodes: 1}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := deriveK8SComputeConfig([]byte(tt.input))
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !errors.Is(err, errNoK8SDerivation) {
				t.Errorf("error %v should wrap errNoK8SDerivation", err)
			}
		})
	}
}

// TestAWSInstanceShapesAreSane guards the hand-maintained table against a
// typo'd zero or a GPU shape with no accelerator type, either of which becomes
// a pod that never schedules.
func TestAWSInstanceShapesAreSane(t *testing.T) {
	for name, shape := range awsInstanceShapes {
		if shape.cpu <= 0 {
			t.Errorf("%s: cpu is %d", name, shape.cpu)
		}
		if shape.memoryBytes <= 0 {
			t.Errorf("%s: memoryBytes is %d", name, shape.memoryBytes)
		}
		if shape.memoryBytes%gib != 0 {
			t.Errorf("%s: memoryBytes %d is not a whole number of GiB", name, shape.memoryBytes)
		}
		if (shape.gpu > 0) != (shape.accelerator != "") {
			t.Errorf(
				"%s: gpu=%d and accelerator=%q must be set together",
				name, shape.gpu, shape.accelerator,
			)
		}
		if shape.accelerator != "" && !documentedAcceleratorTypes[shape.accelerator] {
			t.Errorf(
				"%s: accelerator %q is not a documented ray.io/accelerator-type",
				name, shape.accelerator,
			)
		}
	}
}
