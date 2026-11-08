// Package standards maps the threshold preset names used by the PSV
// frontends to their thresholds.
package standards

import (
	"fmt"
	"strings"

	"github.com/brian-nunez/psv"
)

// Default is the preset used when none is named.
const Default = "wcag22"

// Names lists the presets in display order.
var Names = []string{"wcag22", "trace24", "broadcast", "scene"}

var presets = map[string]func() psv.Thresholds{
	"wcag22":    psv.WCAG22,
	"trace24":   psv.Trace24,
	"broadcast": psv.Broadcast,
	"scene":     psv.SceneChange,
}

// Lookup returns the thresholds of a named preset.
func Lookup(name string) (psv.Thresholds, error) {
	f, ok := presets[name]
	if !ok {
		return psv.Thresholds{}, fmt.Errorf("unknown standard %q (want %s)", name, strings.Join(Names, ", "))
	}

	return f(), nil
}
