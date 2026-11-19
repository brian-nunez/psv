package guardrails

import (
	"slices"
	"testing"
)

var execFiles = []string{
	"media.go",
	"cmd/psv/render.go",
	"examples_test.go",
	"watch_test.go",
	"cmd/psv/main_test.go",
	"internal/server/server_test.go",
}

func TestExecOnlyInListedFiles(t *testing.T) {
	root := repoRoot(t)

	for _, site := range imports(t, root) {
		if site.path == "os/exec" && !slices.Contains(execFiles, site.file) {
			t.Errorf("%s:%d: os/exec is not allowed", site.file, site.pos.Line)
		}
	}
}
