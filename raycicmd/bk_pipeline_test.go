package raycicmd

import (
	"testing"
)

func TestMakeRayDockerPlugin_mountSSHAgent(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		config := &stepDockerPluginConfig{}
		m := makeRayDockerPlugin("test-image:latest", config)
		if _, ok := m["mount-ssh-agent"]; ok {
			t.Errorf(
				"makeRayDockerPlugin() mount-ssh-agent = %#v, want absent",
				m["mount-ssh-agent"],
			)
		}
	})

	t.Run("enabled when configured", func(t *testing.T) {
		config := &stepDockerPluginConfig{mountSSHAgent: true}
		m := makeRayDockerPlugin("test-image:latest", config)
		got, ok := m["mount-ssh-agent"]
		if !ok {
			t.Fatal("makeRayDockerPlugin() missing mount-ssh-agent key")
		}
		if val, ok := got.(bool); !ok || !val {
			t.Errorf(
				"makeRayDockerPlugin() mount-ssh-agent = %#v, want true",
				got,
			)
		}
	})
}

func makeGroup(name string, n int) *bkPipelineGroup {
	steps := make([]any, n)
	for i := range steps {
		steps[i] = map[string]any{"command": "echo"}
	}
	return &bkPipelineGroup{Group: name, Steps: steps}
}

func TestSplitIntoBatches(t *testing.T) {
	// groupJobCount counts the group itself as 1 job plus each step
	// (with parallelism expanding into multiple jobs).
	// So makeGroup("x", N) produces a group with N+1 jobs.

	t.Run("all groups fit in one batch", func(t *testing.T) {
		p := &bkPipeline{
			Steps: []*bkPipelineGroup{
				makeGroup("a", 100), // 101 jobs
				makeGroup("b", 200), // 201 jobs
			},
			Notify: []*bkNotify{{Email: "a@b.com"}},
		}
		batches, err := p.splitIntoBatches(500)
		if err != nil {
			t.Fatalf("splitIntoBatches() error = %v", err)
		}
		if got := len(batches); got != 1 {
			t.Fatalf("len(batches) = %d, want 1", got)
		}
		if got := batches[0].totalJobs(); got != 302 {
			t.Errorf("batch[0].totalJobs() = %d, want 302", got)
		}
		if len(batches[0].Notify) != 1 {
			t.Errorf("batch[0].Notify = %v, want 1 entry", batches[0].Notify)
		}
	})

	t.Run("splits across multiple batches", func(t *testing.T) {
		p := &bkPipeline{
			Steps: []*bkPipelineGroup{
				makeGroup("a", 300), // 301 jobs
				makeGroup("b", 300), // 301 jobs
				makeGroup("c", 100), // 101 jobs
			},
			Notify: []*bkNotify{{Email: "a@b.com"}},
		}
		// a=301 fits alone, b=301 doesn't fit with a (602>500),
		// b+c = 301+101 = 402 fits together.
		batches, err := p.splitIntoBatches(500)
		if err != nil {
			t.Fatalf("splitIntoBatches() error = %v", err)
		}
		if got := len(batches); got != 2 {
			t.Fatalf("len(batches) = %d, want 2", got)
		}
		if got := batches[0].totalJobs(); got != 301 {
			t.Errorf("batch[0].totalJobs() = %d, want 301", got)
		}
		if got := batches[1].totalJobs(); got != 402 {
			t.Errorf("batch[1].totalJobs() = %d, want 402", got)
		}
		if len(batches[0].Notify) != 1 {
			t.Errorf("batch[0] should have Notify")
		}
		if batches[1].Notify != nil {
			t.Errorf("batch[1] should not have Notify")
		}
	})

	t.Run("group exceeds limit", func(t *testing.T) {
		p := &bkPipeline{
			Steps: []*bkPipelineGroup{
				makeGroup("big", 500), // 501 jobs (500 steps + 1 group)
			},
		}
		_, err := p.splitIntoBatches(500)
		if err == nil {
			t.Fatal("splitIntoBatches() expected error for oversized group")
		}
	})

	t.Run("empty pipeline", func(t *testing.T) {
		p := &bkPipeline{}
		batches, err := p.splitIntoBatches(500)
		if err != nil {
			t.Fatalf("splitIntoBatches() error = %v", err)
		}
		if got := len(batches); got != 1 {
			t.Fatalf("len(batches) = %d, want 1", got)
		}
	})

	t.Run("parallelism counted as jobs", func(t *testing.T) {
		p := &bkPipeline{
			Steps: []*bkPipelineGroup{
				{
					Group: "a",
					Steps: []any{
						map[string]any{"command": "echo", "parallelism": 4},
						map[string]any{"command": "echo"},
					},
				},
				{
					Group: "b",
					Steps: []any{
						map[string]any{"command": "echo", "parallelism": 3},
					},
				},
			},
		}
		// Group a = 1 (group) + 4 + 1 = 6 jobs
		// Group b = 1 (group) + 3 = 4 jobs, total = 10
		batches, err := p.splitIntoBatches(8)
		if err != nil {
			t.Fatalf("splitIntoBatches() error = %v", err)
		}
		if got := len(batches); got != 2 {
			t.Fatalf("len(batches) = %d, want 2", got)
		}
		if got := batches[0].totalJobs(); got != 6 {
			t.Errorf("batch[0].totalJobs() = %d, want 6", got)
		}
		if got := batches[1].totalJobs(); got != 4 {
			t.Errorf("batch[1].totalJobs() = %d, want 4", got)
		}
	})

	t.Run("exact limit boundary", func(t *testing.T) {
		p := &bkPipeline{
			Steps: []*bkPipelineGroup{
				makeGroup("a", 499), // 500 jobs (499 steps + 1 group)
				makeGroup("b", 1),   // 2 jobs (1 step + 1 group)
			},
		}
		batches, err := p.splitIntoBatches(500)
		if err != nil {
			t.Fatalf("splitIntoBatches() error = %v", err)
		}
		if got := len(batches); got != 2 {
			t.Fatalf("len(batches) = %d, want 2", got)
		}
		if got := batches[0].totalJobs(); got != 500 {
			t.Errorf("batch[0].totalJobs() = %d, want 500", got)
		}
		if got := batches[1].totalJobs(); got != 2 {
			t.Errorf("batch[1].totalJobs() = %d, want 2", got)
		}
	})
}

func TestBkPipelineTotalSteps(t *testing.T) {
	p := &bkPipeline{
		Steps: []*bkPipelineGroup{
			{
				Group: "group1",
				Steps: []any{
					map[string]any{"command": "echo step1"},
					map[string]any{"command": "echo step2"},
				},
			},
			{
				Group: "group2",
				Steps: []any{
					map[string]any{"command": "echo step3"},
				},
			},
		},
	}
	if total := p.totalSteps(); total != 3 {
		t.Errorf("totalSteps() = %d; want 3", total)
	}
}
