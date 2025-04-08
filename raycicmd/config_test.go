package raycicmd

import (
	"testing"

	"os"
	"path/filepath"
)

func TestLoadConfigFromFile(t *testing.T) {
	tmp := t.TempDir()
	const bs = `ci_temp: "s3://fake/ci-temp"`
	file := filepath.Join(tmp, "config.yaml")
	if err := os.WriteFile(file, []byte(bs), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	config, err := loadConfigFromFile(file)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	const ciTemp = "s3://fake/ci-temp"
	if config.CITemp != ciTemp {
		t.Errorf("config got %q, want %q", config.CITemp, ciTemp)
	}
}

func TestBuilderAgent(t *testing.T) {
	c := &config{
		BuilderQueues: map[string]string{
			"builder": "mybuilder",
			"other":   "otherbuilder",
		},
	}

	q := builderAgent(c, "builder")
	if q != "mybuilder" {
		t.Errorf("builder agent got %q, want `mybuilder`", q)
	}
	q = builderAgent(c, "other")
	if q != "otherbuilder" {
		t.Errorf("builder agent got %q, want `otherbuilder`", q)
	}
}
