package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
)

func main() {
	var total int
	for _, root := range []string{"cmd", "internal"} {
		must(filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			total += scoreFile(file)
			return nil
		}))
	}
	fmt.Println(total)
}

func scoreFile(file *ast.File) int {
	visitor := scorer{}
	ast.Walk(&visitor, file)
	return visitor.total
}

type scorer struct{ total int }

func (s *scorer) Visit(node ast.Node) ast.Visitor {
	if node != nil && s.counts(node) {
		s.total++
	}
	return s
}

func (s *scorer) counts(node ast.Node) bool {
	// Names, literals, declarations, blocks, and statement wrappers are free.
	// The score reflects executable operators and control-flow structure.
	switch node.(type) {
	case *ast.BinaryExpr, *ast.UnaryExpr, *ast.IndexExpr, *ast.IndexListExpr,
		*ast.SliceExpr, *ast.TypeAssertExpr, *ast.CompositeLit,
		*ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt,
		*ast.TypeSwitchStmt, *ast.SelectStmt, *ast.CaseClause, *ast.CommClause,
		*ast.BranchStmt, *ast.GoStmt, *ast.DeferStmt, *ast.SendStmt,
		*ast.CallExpr, *ast.SelectorExpr:
		return true
	default:
		return false
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
