package raycicmd

import (
	"testing"
)

func TestAnyCoreChange(t *testing.T) {
	isEmptyCoreChange := affectedByChange("core", []string{})
	if !isEmptyCoreChange {
		t.Errorf("affectedByChange: got %t, want %t", isEmptyCoreChange, true)
	}
	isCoreCoreChange := affectedByChange("core", []string{"python/abc.py", "python/ray/air/abc.py"})
	if !isCoreCoreChange {
		t.Errorf("affectedByChange: got %t, want %t", isCoreCoreChange, true)
	}
	isAirCoreChange := affectedByChange("core", []string{"python/ray/air/abc.py"})
	if isAirCoreChange {
		t.Errorf("affectedByChange: got %t, want %t", isAirCoreChange, false)
	}
	isNoOwnerChange := affectedByChange("", []string{"python/abc.py", "python/ray/air/abc.py"})
	if !isNoOwnerChange {
		t.Errorf("affectedByChange: got %t, want %t", isNoOwnerChange, true)
	}
}

func TestIsCoreChange(t *testing.T) {
	isCoreCoreChange := isCoreChange("python/abc.py")
	if !isCoreCoreChange {
		t.Errorf("isCoreChange: got %t, want %t", isCoreCoreChange, true)
	}
	isDashboardCoreChange := isCoreChange("dashboard/abc.py")
	if !isDashboardCoreChange {
		t.Errorf("isCoreChange: got %t, want %t", isDashboardCoreChange, true)
	}
	isAirCoreChange := isCoreChange("python/ray/air/abc.py")
	if isAirCoreChange {
		t.Errorf("isCoreChange: got %t, want %t", isAirCoreChange, false)
	}
}
