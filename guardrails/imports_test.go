package guardrails

import (
	"slices"
	"strings"
	"testing"
)

var allowedImports []string

func TestOnlyStdlibAndModuleImports(t *testing.T) {
	root := repoRoot(t)
	module := modulePath(t, root)

	for _, site := range imports(t, root) {
		if isStdlib(site.path) || inModule(site.path, module) || slices.Contains(allowedImports, site.path) {
			continue
		}

		t.Errorf("%s:%d: %q is not in the standard library or this module; "+
			"a third-party import needs approval in guardrails/imports_test.go", site.file, site.pos.Line, site.path)
	}
}

func isStdlib(path string) bool {
	first, _, _ := strings.Cut(path, "/")

	return !strings.Contains(first, ".")
}

func inModule(path, module string) bool {
	return path == module || strings.HasPrefix(path, module+"/")
}
