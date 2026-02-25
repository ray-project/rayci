package raycilint

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

type config struct {
	Filelength *filelengthConfig `yaml:"go_filelength"`
	Coverage   *coverageConfig   `yaml:"go_coverage"`
}

type filelengthConfig struct {
	MaxLines int `yaml:"max_lines"`
}

type coverageConfig struct {
	MinCoveragePct float64 `yaml:"min_coverage_pct"`
}

const defaultConfigPath = ".buildkite/raycilint.yaml"

type multiFlag []string

func (f *multiFlag) String() string { return strings.Join(*f, ", ") }
func (f *multiFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

// overrideKeysHelp returns a formatted list of overridable
// (scalar) YAML keys for the given config struct, suitable for
// embedding in usage text.
func overrideKeysHelp(v interface{}) string {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	var lines []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		switch f.Type.Kind() {
		case reflect.Slice, reflect.Map, reflect.Struct:
			continue
		}
		tag := f.Tag.Get("yaml")
		if tag == "" || tag == "-" {
			continue
		}
		if idx := strings.Index(tag, ","); idx != -1 {
			tag = tag[:idx]
		}
		lines = append(lines, fmt.Sprintf(
			"      %s (%s)", tag, f.Type.Kind(),
		))
	}
	return strings.Join(lines, "\n")
}

func parseOverride(kv string) (string, string, error) {
	parts := strings.SplitN(kv, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf(
			"bad -config-value %q: expected key=value", kv,
		)
	}
	return parts[0], parts[1], nil
}

func loadConfig(path string) (*config, error) {
	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := newConfig()
	if err := yaml.Unmarshal(bs, cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return cfg, nil
}

func newConfig() *config {
	return &config{
		Filelength: &filelengthConfig{},
		Coverage:   &coverageConfig{},
	}
}

func applyOverrides(target interface{}, overrides []string) error {
	v := reflect.ValueOf(target).Elem()
	t := v.Type()

	for _, kv := range overrides {
		key, val, err := parseOverride(kv)
		if err != nil {
			return err
		}
		idx := -1
		for i := 0; i < t.NumField(); i++ {
			tag := t.Field(i).Tag.Get("yaml")
			if j := strings.Index(tag, ","); j != -1 {
				tag = tag[:j]
			}
			if tag == key {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("unknown config key %q", key)
		}
		field := v.Field(idx)
		switch field.Kind() {
		case reflect.Int:
			n, err := strconv.Atoi(val)
			if err != nil {
				return fmt.Errorf(
					"parse %q for %s: %w", val, key, err,
				)
			}
			field.SetInt(int64(n))
		case reflect.Float64:
			f, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return fmt.Errorf(
					"parse %q for %s: %w", val, key, err,
				)
			}
			field.SetFloat(f)
		case reflect.String:
			field.SetString(val)
		default:
			return fmt.Errorf(
				"unsupported type %s for key %q",
				field.Kind(), key,
			)
		}
	}
	return nil
}
