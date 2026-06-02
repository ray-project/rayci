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

	headNode, ok := cc["head_node"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("compute config is missing the required 'head_node'")
	}

	workerNodes := toSlice(cc["worker_nodes"])
	autoSelect := toBool(cc["auto_select_worker_config"])
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
	if f, ok := cc["flags"].(map[string]any); ok {
		for k, v := range f {
			flags[k] = v
		}
	}
	flags["allow-cross-zone-autoscaling"] = toBool(cc["enable_cross_zone_scaling"])
	if v := cc["min_resources"]; isTruthy(v) {
		flags["min_resources"] = v
	}
	if v := cc["max_resources"]; isTruthy(v) {
		flags["max_resources"] = v
	}
	legacy["flags"] = flags

	// Omitted/empty zones == "any" on the backend, so only set when specific.
	if z := toSlice(cc["zones"]); len(z) > 0 {
		legacy["allowed_azs"] = z
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
	name := toString(w["name"])
	if name == "" {
		name = toString(w["instance_type"])
	}
	if name == "" {
		return nil, fmt.Errorf("worker node group must specify 'name' or 'instance_type'")
	}
	legacy["name"] = name

	minWorkers, err := intOrDefault(w["min_nodes"], 0)
	if err != nil {
		return nil, fmt.Errorf("worker node group 'min_nodes': %w", err)
	}
	legacy["min_workers"] = minWorkers
	maxWorkers, err := intOrDefault(w["max_nodes"], 10)
	if err != nil {
		return nil, fmt.Errorf("worker node group 'max_nodes': %w", err)
	}
	legacy["max_workers"] = maxWorkers

	market := toString(w["market_type"])
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
		return fmt.Errorf("unrecognized key(s) %v in template %s", unknown, context)
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

func toSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

func toBool(v any) bool {
	b, _ := v.(bool)
	return b
}

func toString(v any) string {
	s, _ := v.(string)
	return s
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
		return int(t), nil
	default:
		return 0, fmt.Errorf("expected an integer, got %T", v)
	}
}
