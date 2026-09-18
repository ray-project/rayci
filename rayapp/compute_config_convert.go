package rayapp

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"gopkg.in/inf.v0"
	yaml "gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/resource"
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
	topFlags, err := mapField(cc, "flags")
	if err != nil {
		return nil, err
	}
	for k, fv := range topFlags {
		flags[k] = fv
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
	legacy, err := convertNodeCommonFields(head)
	if err != nil {
		return nil, err
	}
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
	legacy, err := convertNodeCommonFields(w)
	if err != nil {
		return nil, err
	}

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
		return nil, fmt.Errorf(
			"worker node group 'min_nodes' (%d) must be non-negative",
			minWorkers,
		)
	}
	legacy["min_workers"] = minWorkers
	maxWorkers, err := intOrDefault(w["max_nodes"], 10)
	if err != nil {
		return nil, fmt.Errorf("worker node group 'max_nodes': %w", err)
	}
	if maxWorkers < 0 {
		return nil, fmt.Errorf(
			"worker node group 'max_nodes' (%d) must be non-negative",
			maxWorkers,
		)
	}
	if maxWorkers < minWorkers {
		return nil, fmt.Errorf(
			"worker node group max_nodes (%d) must be >= min_nodes (%d)",
			maxWorkers,
			minWorkers,
		)
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
func convertNodeCommonFields(node map[string]any) (map[string]any, error) {
	legacy := map[string]any{}
	for _, k := range []string{"instance_type", "required_resources", "labels", "required_labels"} {
		if v, ok := node[k]; ok && v != nil {
			legacy[k] = v
		}
	}
	if rr, ok := legacy["required_resources"]; ok {
		normalized, err := normalizeRequiredResources(rr)
		if err != nil {
			return nil, err
		}
		legacy["required_resources"] = normalized
	}
	if adv := node["advanced_instance_config"]; isTruthy(adv) {
		// Generic key; the launch path prefers it over aws_/gcp_ ones.
		legacy["advanced_configurations_json"] = adv
	}
	// flags is copied through as-is, so validate it is a mapping (fail loud, like other fields).
	fl, err := mapField(node, "flags")
	if err != nil {
		return nil, err
	}
	if len(fl) > 0 {
		legacy["flags"] = fl
	}
	// cloud_deployment is meaningless for a template clone; dropped.
	return legacy, nil
}

// normalizeRequiredResources returns required_resources with `memory` as an
// integer number of bytes.
//
// The API takes memory as bytes, but the user-facing schema also allows a
// Kubernetes quantity string ("8Gi"). The SDK converts on the way to the API
// (PhysicalResources.to_dict(for_api=True) -> _parse_memory_string); the
// published bundle never touches the SDK — the console clone path parses it
// straight into the backend's PhysicalResources, whose `memory` is an int —
// so a string that passes `compute-config create -f` 422s at launch unless we
// convert here too.
func normalizeRequiredResources(v any) (any, error) {
	rr, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("'required_resources' must be a mapping, got %T", v)
	}
	mem, ok := rr["memory"]
	if !ok || mem == nil {
		return rr, nil
	}
	bytes, err := parseMemoryBytes(mem)
	if err != nil {
		return nil, fmt.Errorf("'required_resources.memory': %w", err)
	}
	// Copy rather than mutate: the caller's parsed YAML is not ours to edit.
	out := make(map[string]any, len(rr))
	for k, val := range rr {
		out[k] = val
	}
	out["memory"] = bytes
	return out, nil
}

// parseMemoryBytes converts a memory value to bytes, accepting either an
// integer (already bytes) or a Kubernetes quantity string.
func parseMemoryBytes(v any) (int64, error) {
	switch t := v.(type) {
	case int:
		return nonNegativeBytes(int64(t))
	case int64:
		return nonNegativeBytes(t)
	case float64:
		// YAML gives a float only when the value was written as one; bytes are
		// whole, so a fraction is a mistake rather than something to round.
		if t != math.Trunc(t) {
			return 0, fmt.Errorf("expected a whole number of bytes, got %v", t)
		}
		if t > math.MaxInt64 || t < math.MinInt64 {
			return 0, fmt.Errorf("value %v is out of range", t)
		}
		return nonNegativeBytes(int64(t))
	case string:
		return parseMemoryQuantity(t)
	default:
		return 0, fmt.Errorf(
			"expected an integer number of bytes or a quantity string "+
				"like \"8Gi\", got %T", v,
		)
	}
}

// nonNegativeBytes rejects a negative byte count. Nothing downstream does:
// the backend's PhysicalResources only range-checks int64, so it would reach
// launch as a nonsense pod request.
func nonNegativeBytes(b int64) (int64, error) {
	if b < 0 {
		return 0, fmt.Errorf("must not be negative, got %d bytes", b)
	}
	return b, nil
}

// parseMemoryQuantity converts a Kubernetes quantity string to bytes, using
// the same parser Kubernetes itself uses so the accepted grammar cannot drift
// from the one the docs describe.
//
// The value must come out as a whole number of bytes. Kubernetes tolerates a
// fractional one -- "100m" is 100 *milli*bytes -- but, as its docs put it,
// "this isn't useful to specify since you must always assign whole numbers of
// bytes", and the backend's PhysicalResources.memory is an int, so a fraction
// could not survive the trip anyway. Requiring it whole also removes the one
// place this parser and the SDK's disagree: ParseQuantity().Value() rounds up
// where the SDK's int(Decimal) truncates, and that difference only exists for
// values we now reject.
func parseMemoryQuantity(s string) (int64, error) {
	// ParseQuantity reads a bare suffix ("Gi", "k", ".") as zero, so a typo
	// would silently become no memory at all. The SDK rejects those, and so
	// do we.
	if !strings.ContainsAny(s, "0123456789") {
		return 0, fmt.Errorf("invalid memory quantity %q: no number in it", s)
	}
	q, err := resource.ParseQuantity(s)
	if err != nil {
		return 0, fmt.Errorf("invalid memory quantity %q: %w", s, err)
	}
	if q.Sign() < 0 {
		return 0, fmt.Errorf("memory quantity %q must not be negative", s)
	}

	dec := q.AsDec()
	whole := new(inf.Dec).Round(dec, 0, inf.RoundDown)
	if whole.Cmp(dec) != 0 {
		return 0, fmt.Errorf(
			"memory quantity %q is %s bytes, which is not a whole number of bytes",
			s, dec.String(),
		)
	}
	b, ok := whole.Unscaled()
	if !ok || b == math.MaxInt64 {
		// Quantity saturates at int64 max rather than reporting an overflow,
		// so a value that lands exactly there is treated as out of range.
		return 0, fmt.Errorf("memory quantity %q is too large", s)
	}
	return b, nil
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

// mapField returns the mapping at key (nil if absent), erroring if present but not a mapping.
func mapField(m map[string]any, key string) (map[string]any, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	mm, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%q must be a mapping, got %T", key, v)
	}
	return mm, nil
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
