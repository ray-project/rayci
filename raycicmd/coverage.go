package raycicmd
import (
	"strings"
)

func anyCoreChange(changes []string) bool {
	if len(changes) == 0 {
		// not a pr, run all tests 
		return true
	}
	for _, file := range changes {
		if isCoreChange(file) {
			return true
		}
	}
	return false
}

func isCoreChange(file string) bool {
	return strings.HasPrefix(file, "dashboard") || 
		(strings.HasPrefix(file, "python/") && 
		!strings.HasPrefix(file, "python/ray/air/") && 
		!strings.HasPrefix(file, "python/ray/data/") && 
		!strings.HasPrefix(file, "python/ray/workflow/") && 
		!strings.HasPrefix(file, "python/ray/tune/") && 
		!strings.HasPrefix(file, "python/ray/train/") && 
		!strings.HasPrefix(file, "python/ray/serve/"))
}
