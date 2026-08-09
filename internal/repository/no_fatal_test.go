package repository

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoriesDoNotTerminateProcess(t *testing.T) {
	forEachRepositoryCall(t, func(file string, selector *ast.SelectorExpr) {
		packageName, ok := selector.X.(*ast.Ident)
		if !ok || packageName.Name != "log" {
			return
		}

		if selector.Sel.Name == "Fatal" || selector.Sel.Name == "Fatalf" || selector.Sel.Name == "Fatalln" {
			t.Errorf("%s calls log.%s; repositories must return request errors", file, selector.Sel.Name)
		}
	})
}

func TestRepositoriesDoNotUseSaveForUpdates(t *testing.T) {
	forEachRepositoryCall(t, func(file string, selector *ast.SelectorExpr) {
		if selector.Sel.Name == "Save" {
			t.Errorf("%s calls Save; repositories must use scoped Updates with allowlisted fields", file)
		}
	})
}

func TestOrderRepositoryUsesTransactionsForWrites(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "order_repository.go", nil, 0)
	if err != nil {
		t.Fatalf("parse order_repository.go: %v", err)
	}

	functionsUsingTransaction := map[string]bool{}
	ast.Inspect(parsed, func(node ast.Node) bool {
		function, ok := node.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			return true
		}
		if function.Name.Name != "Add" && function.Name.Name != "Update" {
			return true
		}

		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if selector.Sel.Name == "Transaction" {
				functionsUsingTransaction[function.Name.Name] = true
			}
			return true
		})

		return false
	})

	for _, functionName := range []string{"Add", "Update"} {
		if !functionsUsingTransaction[functionName] {
			t.Fatalf("orderRepository.%s must use db.Transaction", functionName)
		}
	}
}

func TestOrderRepositoryUpdateReplacesItems(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "order_repository.go", nil, 0)
	if err != nil {
		t.Fatalf("parse order_repository.go: %v", err)
	}

	var updateFunction *ast.FuncDecl
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Name.Name == "Update" {
			updateFunction = function
			break
		}
	}
	if updateFunction == nil {
		t.Fatal("orderRepository.Update not found")
	}

	calls := map[string]bool{}
	ast.Inspect(updateFunction.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if selector.Sel.Name == "Delete" || selector.Sel.Name == "Create" {
			calls[selector.Sel.Name] = true
		}
		return true
	})

	for _, name := range []string{"Delete", "Create"} {
		if !calls[name] {
			t.Fatalf("orderRepository.Update must call %s to replace order items", name)
		}
	}
}

func forEachRepositoryCall(t *testing.T, visit func(file string, selector *ast.SelectorExpr)) {
	t.Helper()

	files, err := filepath.Glob("*_repository.go")
	if err != nil {
		t.Fatalf("glob repository files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no repository files found")
	}

	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}

		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}

		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			visit(file, selector)
			return true
		})
	}
}
