package guardrails

import (
	"errors"
	"go/build"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type importSite struct {
	path string
	file string
	pos  token.Position
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the guardrails directory")
		}

		dir = parent
	}
}

func modulePath(t *testing.T, root string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}

	for line := range strings.Lines(string(data)) {
		if path, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(path)
		}
	}

	t.Fatal("go.mod has no module line")

	return ""
}

func walkDirs(t *testing.T, root string, visit func(dir string)) {
	t.Helper()

	err := filepath.WalkDir(root, func(dir string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}

		if dir != root && skipDir(d.Name()) {
			return filepath.SkipDir
		}

		visit(dir)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func goFiles(t *testing.T, root string) []string {
	t.Helper()

	var files []string

	walkDirs(t, root, func(dir string) {
		matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}

		files = append(files, matches...)
	})

	return files
}

func imports(t *testing.T, root string) []importSite {
	t.Helper()

	var sites []importSite

	walkDirs(t, root, func(dir string) {
		pkg, err := build.ImportDir(dir, 0)
		if errors.As(err, new(*build.NoGoError)) {
			return
		}

		if err != nil {
			t.Fatal(err)
		}

		for _, positions := range []map[string][]token.Position{
			pkg.ImportPos, pkg.TestImportPos, pkg.XTestImportPos,
		} {
			for path, at := range positions {
				for _, pos := range at {
					sites = append(sites, importSite{path: path, file: relPath(t, root, pos.Filename), pos: pos})
				}
			}
		}
	})

	return sites
}

func relPath(t *testing.T, root, file string) string {
	t.Helper()

	rel, err := filepath.Rel(root, file)
	if err != nil {
		t.Fatal(err)
	}

	return filepath.ToSlash(rel)
}

func skipDir(name string) bool {
	return name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}
