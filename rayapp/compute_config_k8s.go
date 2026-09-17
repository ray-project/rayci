package rayapp

import (
	"errors"
	"fmt"

	yaml "gopkg.in/yaml.v3"
)

// Derive a declarative K8S compute config from an instance-typed one.
//
// A K8s-stack cloud cannot honor a named instance type: those are per-cluster
// registrations, while `required_resources` resolves server-side into a free
// pod. A template asks for one with `K8S: auto` in BUILD.yaml; one that names
// a hand-written k8s.yaml keeps it, and one that mentions no K8S key gets no
// K8S config.
//
// "Declarative" is not a flag anywhere -- a node is declarative when it
// carries required_resources instead of instance_type (ComputeNodeType holds
// both as optional: "Optional when using free pod shapes with
// required_resources"). So the translation is decided per node, on what the
// node actually carries, not on which file it came from.

// The compute config keys used in BUILD.yaml, and the value that asks for a
// derived config rather than naming a file.
const (
	awsComputeConfigKey = "AWS"
	k8sComputeConfigKey = "K8S"
	k8sAutoValue        = "auto"
)

// errNoK8SDerivation reports that a config cannot be translated. Deriving is
// opt-in per template (`K8S: auto`), so the builder surfaces these as build
// failures: the template asked for a config it is not going to get.
var errNoK8SDerivation = errors.New("cannot derive a K8S compute config")

// instanceShape is what a node of a given instance type asks for as a pod.
type instanceShape struct {
	cpu         int
	memoryBytes int64
	gpu         int
	accelerator string // ray.io/accelerator-type, empty for CPU-only
}

const gib = int64(1) << 30

// documentedAcceleratorTypes are the ray.io/accelerator-type values the
// declarative docs accept. Anything else is rejected at launch, so the table
// below may only use these.
var documentedAcceleratorTypes = keySet(
	"T4", "A10G", "A100", "L4", "H100",
	"TPU-V4", "TPU-V5E", "TPU-V6E",
)

// awsInstanceShapes covers the instance types used by templates today. It is
// hand-maintained against the published AWS specs; an unlisted type is
// reported rather than guessed, because a wrong shape here becomes a pod that
// never schedules.
var awsInstanceShapes = map[string]instanceShape{
	"m5.2xlarge":    {cpu: 8, memoryBytes: 32 * gib},
	"m5.4xlarge":    {cpu: 16, memoryBytes: 64 * gib},
	"g4dn.xlarge":   {cpu: 4, memoryBytes: 16 * gib, gpu: 1, accelerator: "T4"},
	"g4dn.12xlarge": {cpu: 48, memoryBytes: 192 * gib, gpu: 4, accelerator: "T4"},
	"g5.xlarge":     {cpu: 4, memoryBytes: 16 * gib, gpu: 1, accelerator: "A10G"},
	"g5.2xlarge":    {cpu: 8, memoryBytes: 32 * gib, gpu: 1, accelerator: "A10G"},
	"g5.4xlarge":    {cpu: 16, memoryBytes: 64 * gib, gpu: 1, accelerator: "A10G"},
	"g6.2xlarge":    {cpu: 8, memoryBytes: 32 * gib, gpu: 1, accelerator: "L4"},
	"g6.4xlarge":    {cpu: 16, memoryBytes: 64 * gib, gpu: 1, accelerator: "L4"},
	"g6.12xlarge":   {cpu: 48, memoryBytes: 192 * gib, gpu: 4, accelerator: "L4"},
	"g6.24xlarge":   {cpu: 96, memoryBytes: 384 * gib, gpu: 4, accelerator: "L4"},
}

// An unschedulable head runs the coordinator and nothing else, so it is sized
// for that rather than for the instance type the VM config names -- the same
// sizing the hand-written pilot configs use.
const (
	coordinatorCPU         = 4
	coordinatorMemoryBytes = 8 * gib
)

// deriveK8SComputeConfig translates an instance-typed compute config into a
// declarative one. Nodes that are already declarative pass through untouched.
func deriveK8SComputeConfig(data []byte) ([]byte, error) {
	legacy, err := isLegacyComputeConfigData(data)
	if err != nil {
		return nil, err
	}
	if legacy {
		// The legacy schema is an output format; templates author the new one.
		return nil, fmt.Errorf("%w: source config uses the legacy schema", errNoK8SDerivation)
	}

	var cc map[string]any
	if err := yaml.Unmarshal(data, &cc); err != nil {
		return nil, fmt.Errorf("parse compute config: %w", err)
	}

	autoSelect, err := boolField(cc, "auto_select_worker_config")
	if err != nil {
		return nil, err
	}

	headVal, ok := cc["head_node"]
	if !ok || headVal == nil {
		return nil, fmt.Errorf("%w: no head_node", errNoK8SDerivation)
	}
	head, ok := headVal.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: head_node is not a mapping", errNoK8SDerivation)
	}

	// The head's own instance type describes the machine the template was
	// sized for, and an auto-select cluster's worker group is derived from it
	// below -- but only an instance-typed head has one to read.
	var headInstanceShape instanceShape
	if !nodeIsDeclarative(head) {
		headInstanceShape, err = shapeOfNode(head)
		if err != nil {
			return nil, err
		}
	}

	// A head pinned to CPU: 0 only coordinates, so give it coordinator sizing
	// rather than the whole machine.
	headShape := instanceShape{cpu: coordinatorCPU, memoryBytes: coordinatorMemoryBytes}
	schedulable, err := headIsSchedulable(head)
	if err != nil {
		return nil, err
	}
	if schedulable {
		headShape = headInstanceShape
	}
	newHead, err := declarativeNode(head, headShape)
	if err != nil {
		return nil, err
	}
	cc["head_node"] = newHead

	workers, err := sliceField(cc, "worker_nodes")
	if err != nil {
		return nil, err
	}

	// Auto-select has no declarative form: the backend picks worker shapes from
	// the cloud's registered instance types, which a K8s cloud does not have.
	// Stand in one group shaped like the machine the head names -- the same
	// substitution the hand-written single-node config makes. This is the only
	// shape here that is invented rather than translated, which is a reason to
	// prefer an authored k8s.yaml wherever the choice matters.
	if autoSelect && len(workers) == 0 {
		if nodeIsDeclarative(head) {
			// No instance type on the head, so nothing to size a group from.
			return nil, fmt.Errorf(
				"%w: auto_select_worker_config with a declarative head",
				errNoK8SDerivation,
			)
		}
		delete(cc, "auto_select_worker_config")
		name := "cpu_worker"
		if headInstanceShape.gpu > 0 {
			name = "gpu_worker"
		}
		group := map[string]any{"name": name, "max_nodes": 1}
		derived, err := declarativeNode(group, headInstanceShape)
		if err != nil {
			return nil, err
		}
		cc["worker_nodes"] = []any{derived}
		out, err := yaml.Marshal(cc)
		if err != nil {
			return nil, fmt.Errorf("marshal derived K8S compute config: %w", err)
		}
		return out, nil
	}

	if len(workers) == 0 {
		if !schedulable {
			// Nothing could run: no workers, no auto-select, unschedulable head.
			return nil, fmt.Errorf(
				"%w: unschedulable head with no worker_nodes", errNoK8SDerivation,
			)
		}
		// A head-only cluster is complete as it stands.
		out, err := yaml.Marshal(cc)
		if err != nil {
			return nil, fmt.Errorf("marshal derived K8S compute config: %w", err)
		}
		return out, nil
	}
	newWorkers := make([]any, 0, len(workers))
	for _, w := range workers {
		wm, ok := w.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: worker_nodes entry is not a mapping", errNoK8SDerivation)
		}
		// Ask what the node carries before reading an instance type off it.
		var shape instanceShape
		if !nodeIsDeclarative(wm) {
			shape, err = shapeOfNode(wm)
			if err != nil {
				return nil, err
			}
		}
		nw, err := declarativeNode(wm, shape)
		if err != nil {
			return nil, err
		}
		newWorkers = append(newWorkers, nw)
	}
	cc["worker_nodes"] = newWorkers

	out, err := yaml.Marshal(cc)
	if err != nil {
		return nil, fmt.Errorf("marshal derived K8S compute config: %w", err)
	}
	return out, nil
}

// headIsSchedulable reports whether the head runs workloads. Anything other
// than an explicit CPU: 0 counts as schedulable.
func headIsSchedulable(head map[string]any) (bool, error) {
	res, err := mapField(head, "resources")
	if err != nil {
		return false, err
	}
	cpu, ok := res["CPU"]
	if !ok {
		return true, nil
	}
	n, err := intOrDefault(cpu, 0)
	if err != nil {
		return false, fmt.Errorf("head_node resources CPU: %w", err)
	}
	return n != 0, nil
}

// shapeOfNode looks up the pod shape a node's instance type asks for.
func shapeOfNode(node map[string]any) (instanceShape, error) {
	instanceType, err := stringField(node, "instance_type")
	if err != nil {
		return instanceShape{}, err
	}
	if instanceType == "" {
		return instanceShape{}, fmt.Errorf("%w: a node has no instance_type", errNoK8SDerivation)
	}
	shape, ok := awsInstanceShapes[instanceType]
	if !ok {
		return instanceShape{}, fmt.Errorf(
			"%w: instance type %q is not in the shape table (add it to awsInstanceShapes)",
			errNoK8SDerivation, instanceType,
		)
	}
	return shape, nil
}

// nodeIsDeclarative reports whether a node already states what it needs as
// resources rather than naming a machine to run on.
//
// Nothing in the schema flags this: ComputeNodeType carries instance_type and
// required_resources both as optional ("Optional when using free pod shapes
// with required_resources"), and which one is set is the whole difference. So
// the test is field presence, and it has to be asked before anything reads an
// instance type off the node.
func nodeIsDeclarative(node map[string]any) bool {
	rr, ok := node["required_resources"]
	return ok && rr != nil
}

// declarativeNode returns the node with instance_type replaced by the
// equivalent required_resources, leaving every other field alone. A node that
// is already declarative is returned untouched -- it was authored that way and
// we have nothing to add.
func declarativeNode(node map[string]any, shape instanceShape) (map[string]any, error) {
	if nodeIsDeclarative(node) {
		// "Each node group must use either predefined instances or declarative
		// syntax, not both" -- docs.anyscale.com/configuration/compute/declarative.
		// Dropping one silently would be picking for the author.
		if it, err := stringField(node, "instance_type"); err != nil {
			return nil, err
		} else if it != "" {
			return nil, fmt.Errorf(
				"%w: a node sets both instance_type %q and required_resources",
				errNoK8SDerivation, it,
			)
		}
		return node, nil
	}

	out := make(map[string]any, len(node)+1)
	for k, v := range node {
		if k == "instance_type" {
			continue
		}
		out[k] = v
	}

	required := map[string]any{"CPU": shape.cpu, "memory": shape.memoryBytes}
	if shape.gpu > 0 {
		required["GPU"] = shape.gpu
	}
	out["required_resources"] = required

	// The launch-time GPU validation reads the accelerator type off
	// required_labels, never required_resources.accelerator.
	if shape.accelerator != "" {
		labels, err := mapField(node, "required_labels")
		if err != nil {
			return nil, err
		}
		merged := make(map[string]any, len(labels)+1)
		for k, v := range labels {
			merged[k] = v
		}
		if _, ok := merged["ray.io/accelerator-type"]; !ok {
			merged["ray.io/accelerator-type"] = shape.accelerator
		}
		out["required_labels"] = merged
	}

	return out, nil
}
