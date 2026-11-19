package guardrails

import (
	"go/parser"
	"go/token"
	"regexp"
	"testing"
)

var nolintDirective = regexp.MustCompile(`^//\s*nolint`)

func TestNoNolintDirectives(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()

	for _, file := range goFiles(t, root) {
		f, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}

		for _, group := range f.Comments {
			for _, c := range group.List {
				if nolintDirective.MatchString(c.Text) {
					t.Errorf("%s:%d: nolint directives are not allowed",
						relPath(t, root, file), fset.Position(c.Pos()).Line)
				}
			}
		}
	}
}
