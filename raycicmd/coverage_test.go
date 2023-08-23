package raycicmd

import (
	"testing"
)

func TestAnyCoreChange(t *testing.T) {
	isEmptyCoreChange := anyCoreChange([]string{})
	if (!isEmptyCoreChange) {
		t.Errorf("anyCoreChange: got %t, want %t", isEmptyCoreChange, true)
	}
	isCoreCoreChange := anyCoreChange([]string{"python/abc.py", "python/ray/air/abc.py"})
	if (!isCoreCoreChange) {
		t.Errorf("anyCoreChange: got %t, want %t", isCoreCoreChange, true)
	}
	isAirCoreChange := anyCoreChange([]string{"python/ray/air/abc.py"})
	if (isAirCoreChange) {
		t.Errorf("anyCoreChange: got %t, want %t", isAirCoreChange, false)
	}
}

func TestIsCoreChange(t *testing.T) {
	isCoreCoreChange := isCoreChange("python/abc.py")
	if (!isCoreCoreChange) {
		t.Errorf("isCoreChange: got %t, want %t", isCoreCoreChange, true)
	}
	isDashboardCoreChange := isCoreChange("dashboard/abc.py")
	if (!isDashboardCoreChange) {
		t.Errorf("isCoreChange: got %t, want %t", isDashboardCoreChange, true)
	}
	isAirCoreChange := isCoreChange("python/ray/air/abc.py")
	if (isAirCoreChange) {
		t.Errorf("isCoreChange: got %t, want %t", isAirCoreChange, false)
	}
}
