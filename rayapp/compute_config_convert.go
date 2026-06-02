package rayapp

import (
	"fmt"
	"sort"

	yaml "gopkg.in/yaml.v3"
)

// Convert the new user-facing ComputeConfig schema (head_node/worker_nodes/...) to the
// legacy CreateComputeTemplateConfig the console clone path parses, so the backend never
// sees the new schema. Mirrors the anyscale SDK converter (compute_config_sdk.py).
// Legacy/new detection is handled by isLegacyComputeConfigFormat.

var newSchemaTopLevelKeys = keySet(
	"cloud", "cloud_resource", "head_node", "worker_nodes", "zones",
	"enable_cross_zone_scaling", "advanced_instance_config", "min_resources",
	"max_resources", "flags", "auto_select_worker_config",
)

var newSchemaNodeKeys = keySet(
	"instance_type", "resources", "required_resources", "labels", "required_labels",
	"advanced_instance_config", "flags", "cloud_deployment",
)

var newSchemaWorkerNodeKeys = keySet(
	"instance_type", "resources", "required_resources", "labels", "required_labels",
	"advanced_instance_config", "flags", "cloud_deployment",
	"name", "min_nodes", "max_nodes", "market_type",
)

var validMarketTypes = keySet("ON_DEMAND", "SPOT", "PREFER_SPOT")

// convertNewComputeConfigToLegacy parses a new-schema compute config and returns
// the equivalent legacy-schema YAML. Callers must only pass new-schema configs
// (see isLegacyComputeConfigFormat).
func convertNewComputeConfigToLegacy(data []byte) ([]byte, error) {
	var cc map[string]any
	if err := yaml.Unmarshal(data, &cc); err != nil {
		return nil, fmt.Errorf("parse compute config: %w", err)
	}
	if err := rejectUnknownKeys(cc, newSchemaTopLevelKeys, "compute config"); err != nil {
		return nil, err
	}

	headVal, ok := cc["head_node"]
	if !ok || headVal == nil {
		return nil, fmt.Errorf("compute config is missing the required 'head_node'")
	}
	headNode, ok := headVal.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("'head_node' must be a mapping, got %T", headVal)
	}

	workerNodes, err := sliceField(cc, "worker_nodes")
	if err != nil {
		return nil, err
	}
	autoSelect, err := boolField(cc, "auto_select_worker_config")
	if err != nil {
		return nil, err
	}
	// Head is schedulable only when it is the sole node in the cluster.
	schedulableByDefault := len(workerNodes) == 0 && !autoSelect

	headLegacy, err := convertHeadNode(headNode, schedulableByDefault)
	if err != nil {
		return nil, err
	}

	workerLegacy := []any{}
	for _, w := range workerNodes {
		wm, ok := w.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("worker_nodes entry is not a mapping")
		}
		conv, err := convertWorkerNode(wm)
		if err != nil {
			return nil, err
		}
		workerLegacy = append(workerLegacy, conv)
	}

	legacy := map[string]any{
		"head_node_type":            headLegacy,
		"worker_node_types":         workerLegacy,
		"auto_select_worker_config": autoSelect,
	}

	// Cross-zone scaling lives in flags; the SDK always writes it (even when false).
	flags := map[string]any{}
	if v, ok := cc["flags"]; ok && v != nil {
		f, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("'flags' must be a mapping, got %T", v)
		}
		for k, fv := range f {
			flags[k] = fv
		}
	}
	crossZone, err := boolField(cc, "enable_cross_zone_scaling")
	if err != nil {
		return nil, err
	}
	flags["allow-cross-zone-autoscaling"] = crossZone
	if v := cc["min_resources"]; isTruthy(v) {
		flags["min_resources"] = v
	}
	if v := cc["max_resources"]; isTruthy(v) {
		flags["max_resources"] = v
	}
	legacy["flags"] = flags

	// Omitted/empty zones == "any" on the backend, so only set when specific.
	zones, err := sliceField(cc, "zones")
	if err != nil {
		return nil, err
	}
	if len(zones) > 0 {
		legacy["allowed_azs"] = zones
	}
	if adv := cc["advanced_instance_config"]; isTruthy(adv) {
		legacy["advanced_configurations_json"] = adv
	}

	out, err := yaml.Marshal(legacy)
	if err != nil {
		return nil, fmt.Errorf("marshal legacy compute config: %w", err)
	}
	return out, nil
}

func convertHeadNode(head map[string]any, schedulableByDefault bool) (map[string]any, error) {
	if err := rejectUnknownKeys(head, newSchemaNodeKeys, "head_node"); err != nil {
		return nil, err
	}
	legacy := convertNodeCommonFields(head)
	// New schema has no head name; backend requires one (SDK uses "head-node").
	legacy["name"] = "head-node"
	if res, ok := head["resources"]; ok && res != nil {
		legacy["resources"] = res
	} else if !schedulableByDefault {
		// Workers/auto-select present -> pin head unschedulable (SDK default).
		legacy["resources"] = map[string]any{"CPU": 0, "GPU": 0}
	}
	return legacy, nil
}

func convertWorkerNode(w map[string]any) (map[string]any, error) {
	if err := rejectUnknownKeys(w, newSchemaWorkerNodeKeys, "worker_nodes entry"); err != nil {
		return nil, err
	}
	legacy := convertNodeCommonFields(w)

	// Name defaults to the instance type (matches WorkerNodeGroupConfig).
	name, err := stringField(w, "name")
	if err != nil {
		return nil, err
	}
	instanceType, err := stringField(w, "instance_type")
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = instanceType
	}
	if name == "" {
		return nil, fmt.Errorf("worker node group must specify 'name' or 'instance_type'")
	}
	legacy["name"] = name

	minWorkers, err := intOrDefault(w["min_nodes"], 0)
	if err != nil {
		return nil, fmt.Errorf("worker node group 'min_nodes': %w", err)
	}
	if minWorkers < 0 {
		return nil, fmt.Errorf("worker node group 'min_nodes' (%d) must be non-negative", minWorkers)
	}
	legacy["min_workers"] = minWorkers
	maxWorkers, err := intOrDefault(w["max_nodes"], 10)
	if err != nil {
		return nil, fmt.Errorf("worker node group 'max_nodes': %w", err)
	}
	if maxWorkers < 0 {
		return nil, fmt.Errorf("worker node group 'max_nodes' (%d) must be non-negative", maxWorkers)
	}
	if maxWorkers < minWorkers {
		return nil, fmt.Errorf("worker node group max_nodes (%d) must be >= min_nodes (%d)", maxWorkers, minWorkers)
	}
	legacy["max_workers"] = maxWorkers

	market, err := stringField(w, "market_type")
	if err != nil {
		return nil, err
	}
	if market == "" {
		market = "ON_DEMAND"
	}
	if !validMarketTypes[market] {
		return nil, fmt.Errorf("unknown market_type %q in worker node group", market)
	}
	legacy["use_spot"] = market == "SPOT" || market == "PREFER_SPOT"
	legacy["fallback_to_ondemand"] = market == "PREFER_SPOT"

	if res, ok := w["resources"]; ok && res != nil {
		legacy["resources"] = res
	}
	return legacy, nil
}

// convertNodeCommonFields maps fields shared by head and worker nodes (not
// name/resources/scaling/market).
func convertNodeCommonFields(node map[string]any) map[string]any {
	legacy := map[string]any{}
	for _, k := range []string{"instance_type", "required_resources", "labels", "required_labels"} {
		if v, ok := node[k]; ok && v != nil {
			legacy[k] = v
		}
	}
	if adv := node["advanced_instance_config"]; isTruthy(adv) {
		// Generic key; the launch path prefers it over aws_/gcp_ ones.
		legacy["advanced_configurations_json"] = adv
	}
	if fl := node["flags"]; isTruthy(fl) {
		legacy["flags"] = fl
	}
	// cloud_deployment is meaningless for a template clone; dropped.
	return legacy
}

func rejectUnknownKeys(m map[string]any, known map[string]bool, context string) error {
	var unknown []string
	for k := range m {
		if !known[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("unrecognized key(s) %v in %s", unknown, context)
	}
	return nil
}

func keySet(keys ...string) map[string]bool {
	s := make(map[string]bool, len(keys))
	for _, k := range keys {
		s[k] = true
	}
	return s
}

// sliceField returns the list at key (nil if absent), erroring if present but not a list.
func sliceField(m map[string]any, key string) ([]any, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	s, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%q must be a list, got %T", key, v)
	}
	return s, nil
}

// boolField returns the bool at key (false if absent), erroring if present but not a bool.
func boolField(m map[string]any, key string) (bool, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return false, nil
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("%q must be a boolean, got %T", key, v)
	}
	return b, nil
}

// stringField returns the string at key ("" if absent), erroring if present but not a string.
func stringField(m map[string]any, key string) (string, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%q must be a string, got %T", key, v)
	}
	return s, nil
}

func isTruthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		return t != ""
	case []any:
		return len(t) > 0
	case map[string]any:
		return len(t) > 0
	default:
		return true
	}
}

// intOrDefault returns def when v is absent (nil), the int value when v is an
// integer, and an error when v is present but not an integer (fail loud rather
// than silently dropping a misconfigured value).
func intOrDefault(v any, def int) (int, error) {
	switch t := v.(type) {
	case nil:
		return def, nil
	case int:
		return t, nil
	case int64:
		return int(t), nil
	case float64:
		if t != float64(int(t)) {
			return 0, fmt.Errorf("expected an integer, got %v", t)
		}
		return int(t), nil
	default:
		return 0, fmt.Errorf("expected an integer, got %T", v)
	}
}
