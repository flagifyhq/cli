package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"sort"
	"strings"
	"testing"
)

// loadCalls / saveCalls are the config entry points that read and write the
// store. A single scope holding one of each is a read-modify-write split across
// two critical sections: another flagify process can persist a rotated,
// single-use refresh token in between, and the save then reverts it, costing
// the user their session. config.UpdateStore exists so that cannot happen.
var (
	loadCalls = map[string]bool{"Load": true, "LoadStore": true, "LoadOrMigrate": true}
	saveCalls = map[string]bool{"Save": true, "SaveStore": true}
)

// allowedReadModifyWrite is the escape hatch: scopes where a load paired with a
// save is intentional, each with its reason. It is empty on purpose — every
// mutation path currently goes through config.UpdateStore. The completed-login
// paths (loginBrowser, loginDevice) only save; the store read that precedes
// them happens in prepareLoginProfile, and their write is an authoritative
// overwrite that is meant to supersede whatever tokens were on disk.
//
// Limitation worth knowing: this catches a load and a save inside one scope,
// which is the shape every regression has taken. It does not catch a load and a
// save split across two functions passing the store between them.
var allowedReadModifyWrite = map[string]string{}

// TestNoStoreReadModifyWriteOutsideTransaction is the mechanical check that no
// config.json mutation path was left outside config.UpdateStore. It replaces
// trusting a hand-kept list of converted call sites: a new command that loads
// the store and saves it later fails here instead of silently reintroducing the
// lost-update race on a path no test exercises.
func TestNoStoreReadModifyWriteOutsideTransaction(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse cmd package: %v", err)
	}

	var offenders []string
	for _, pkg := range pkgs {
		for path, file := range pkg.Files {
			fileName := path[strings.LastIndex(path, "/")+1:]
			for _, decl := range file.Decls {
				for scope, node := range scopesOf(decl) {
					loads, saves := classifyCalls(node)
					if len(loads) == 0 || len(saves) == 0 {
						continue
					}
					key := fileName + ":" + scope
					if _, ok := allowedReadModifyWrite[key]; ok {
						continue
					}
					offenders = append(offenders, key+
						" loads with "+strings.Join(loads, "/")+
						" and writes with "+strings.Join(saves, "/"))
				}
			}
		}
	}

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Fatalf(
			"these scopes read the store and write it back outside a transaction:\n  %s\n\n"+
				"Wrap the mutation in config.UpdateStore(func(*config.Store) error {...}) so the "+
				"read and the write share one locked critical section. If the write is an "+
				"authoritative overwrite (a completed login), add it to allowedReadModifyWrite "+
				"with the reason.",
			strings.Join(offenders, "\n  "),
		)
	}
}

// scopesOf returns the named function scopes a declaration contributes: a plain
// function, or each function literal assigned in a var block (cobra RunE).
func scopesOf(decl ast.Decl) map[string]ast.Node {
	scopes := map[string]ast.Node{}
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Body != nil {
			scopes[d.Name.Name] = d.Body
		}
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if i < len(vs.Values) {
					scopes[name.Name] = vs.Values[i]
				}
			}
		}
	}
	return scopes
}

// classifyCalls reports which config load and save entry points a node calls.
func classifyCalls(node ast.Node) (loads, saves []string) {
	seen := map[string]bool{}
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "config" {
			return true
		}
		name := sel.Sel.Name
		if seen[name] {
			return true
		}
		seen[name] = true
		switch {
		case loadCalls[name]:
			loads = append(loads, "config."+name)
		case saveCalls[name]:
			saves = append(saves, "config."+name)
		}
		return true
	})
	sort.Strings(loads)
	sort.Strings(saves)
	return loads, saves
}
