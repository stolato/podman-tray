package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// extraKeys are texts translated without a literal in a T or F call.
var extraKeys = []string{
	"requires macOS 13", // returned by loginitem_darwin.m
}

// TestCatalog checks that every text passed to T or F (and every string in a map or
// struct literal named *Messages) has a Portuguese translation with the same verbs, and
// that the catalog has no leftovers.
func TestCatalog(t *testing.T) {
	used := map[string]bool{}
	for _, k := range extraKeys {
		used[k] = true
	}
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "dist" || d.Name() == "bin" || strings.HasPrefix(d.Name(), ".")) && path != root {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				if isTranslateCall(n) && len(n.Args) > 0 {
					// Non-literal arguments come from extraKeys or a *Messages variable.
					if s, ok := stringLit(n.Args[0]); ok {
						used[s] = true
					}
				}
			case *ast.ValueSpec:
				for i, name := range n.Names {
					if strings.HasSuffix(name.Name, "Messages") && i < len(n.Values) {
						ast.Inspect(n.Values[i], func(n ast.Node) bool {
							if kv, ok := n.(*ast.KeyValueExpr); ok {
								ast.Inspect(kv.Value, func(n ast.Node) bool {
									if s, ok := stringLit(n); ok {
										used[s] = true
									}
									return true
								})
								return false
							}
							return true
						})
					}
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	for en := range used {
		tr, ok := pt[en]
		if !ok {
			t.Errorf("missing Portuguese translation for %q", en)
			continue
		}
		if a, b := verbs(en), verbs(tr); !slices.Equal(a, b) {
			t.Errorf("%q: verbs %v, translation %q has %v", en, a, tr, b)
		}
	}
	for en := range pt {
		if !used[en] {
			t.Errorf("unused translation %q", en)
		}
	}
}

func isTranslateCall(c *ast.CallExpr) bool {
	name := exprName(c.Fun)
	return name == "i18n.T" || name == "i18n.F"
}

func exprName(e ast.Expr) string {
	if sel, ok := e.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok {
			return id.Name + "." + sel.Sel.Name
		}
	}
	return ""
}

func stringLit(n ast.Node) (string, bool) {
	lit, ok := n.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

var verbRE = regexp.MustCompile(`%[-+# 0-9.]*[a-zA-Z%]`)

func verbs(s string) []string {
	return verbRE.FindAllString(s, -1)
}

func TestPick(t *testing.T) {
	for lang, want := range map[string]bool{
		"pt-BR": true, "pt_BR.UTF-8": true, "pt": true, "PT-pt": true,
		"en-US": false, "": false, "C": false, "es": false,
	} {
		if got := pick(lang) != nil; got != want {
			t.Errorf("pick(%q) translated = %v, want %v", lang, got, want)
		}
	}
}

func TestFallback(t *testing.T) {
	old := catalog
	defer func() { catalog = old }()

	catalog = nil
	if got := F("%d images removed", 3); got != "3 images removed" {
		t.Errorf("English: %q", got)
	}
	catalog = pt
	if got := F("%d images removed", 3); got != "3 imagens removidas" {
		t.Errorf("Portuguese: %q", got)
	}
	if got := T("not in the catalog"); got != "not in the catalog" {
		t.Errorf("fallback: %q", got)
	}
}
