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
			ast.Inspect(file, func(node ast.Node) bool {
				if node != nil {
					total++
				}
				return true
			})
			return nil
		}))
	}
	fmt.Println(total)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
