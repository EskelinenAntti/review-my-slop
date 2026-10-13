// Command signatures lists Go function and method declarations without bodies.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/scanner"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	err := extractProductionSignaturesCLI(os.Args[1:], os.Stdout)
	if err == flag.ErrHelp {
		return
	}
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func extractProductionSignaturesCLI(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("signatures", flag.ContinueOnError)
	root := flags.String("root", ".", "source tree (excludes Go test files)")
	output := flags.String("output", "", "output file; defaults to stdout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	var signatures bytes.Buffer
	if err := writeGoSignatures(&signatures, *root); err != nil {
		return err
	}
	if *output == "" {
		_, err := signatures.WriteTo(stdout)
		return err
	}
	return os.WriteFile(*output, signatures.Bytes(), 0600)
}

// writeGoSignatures walks in lexical path order, regardless of build tags.
func writeGoSignatures(output io.Writer, root string) error {
	extractor := signatureExtractor{
		output: output, root: root, fset: token.NewFileSet(),
	}
	return filepath.WalkDir(root, extractor.visitSourcePath)
}

type signatureExtractor struct {
	output io.Writer
	root   string
	fset   *token.FileSet
}

func (e signatureExtractor) visitSourcePath(
	path string,
	entry fs.DirEntry,
	walkErr error,
) error {
	if walkErr != nil {
		return walkErr
	}
	if entry.IsDir() {
		return e.skipExcludedDirectory(path, entry.Name())
	}
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return nil
	}
	return e.writeSourceFileSignatures(path)
}

func (e signatureExtractor) skipExcludedDirectory(path, name string) error {
	if path == e.root {
		return nil
	}
	if strings.HasPrefix(name, ".") {
		return filepath.SkipDir
	}
	switch name {
	case "vendor", "node_modules", "testdata", "testutil":
		return filepath.SkipDir
	default:
		return nil
	}
}

func (e signatureExtractor) writeSourceFileSignatures(path string) error {
	file, err := parser.ParseFile(
		e.fset, path, nil, parser.SkipObjectResolution,
	)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(e.root, filepath.Dir(path))
	if err != nil {
		return err
	}
	pkg := filepath.ToSlash(relative) + " (package " + file.Name.Name + ")"
	for _, declaration := range file.Decls {
		if err := e.writeDeclarationSignatures(pkg, declaration); err != nil {
			return err
		}
	}
	return nil
}

func (e signatureExtractor) writeDeclarationSignatures(
	pkg string,
	declaration ast.Decl,
) error {
	switch declaration := declaration.(type) {
	case *ast.FuncDecl:
		declaration.Body, declaration.Doc = nil, nil
		return e.writeSignatureLine(pkg, declaration)
	case *ast.GenDecl:
		return e.writeTypeMethodSignatures(pkg, declaration.Specs)
	default:
		return nil
	}
}

func (e signatureExtractor) writeTypeMethodSignatures(
	pkg string,
	specs []ast.Spec,
) error {
	for _, spec := range specs {
		if typeSpec, ok := spec.(*ast.TypeSpec); ok {
			if err := e.writeInterfaceSignatures(pkg, typeSpec); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e signatureExtractor) writeInterfaceSignatures(
	pkg string,
	typeSpec *ast.TypeSpec,
) error {
	iface, ok := typeSpec.Type.(*ast.InterfaceType)
	if !ok {
		return nil
	}
	for _, method := range iface.Methods.List {
		owner := pkg + " interface " + typeSpec.Name.Name
		if err := e.writeInterfaceMethodSignature(owner, method); err != nil {
			return err
		}
	}
	return nil
}

func (e signatureExtractor) writeInterfaceMethodSignature(
	pkg string,
	method *ast.Field,
) error {
	methodType, ok := method.Type.(*ast.FuncType)
	if !ok {
		return nil // Embedded interfaces have no explicit method declaration.
	}
	for _, name := range method.Names {
		declaration := &ast.FuncDecl{Name: name, Type: methodType}
		if err := e.writeSignatureLine(pkg, declaration); err != nil {
			return err
		}
	}
	return nil
}

func (e signatureExtractor) writeSignatureLine(
	pkg string,
	declaration *ast.FuncDecl,
) error {
	var signature bytes.Buffer
	if err := printer.Fprint(&signature, e.fset, declaration); err != nil {
		return err
	}
	_, err := fmt.Fprintf(
		e.output, "%s: %s\n", pkg, singleLineGoSignature(signature.Bytes()),
	)
	return err
}

// singleLineGoSignature collapses layout while preserving literal contents.
func singleLineGoSignature(source []byte) string {
	fset := token.NewFileSet()
	file := fset.AddFile("signature", -1, len(source))
	var lexer scanner.Scanner
	lexer.Init(file, source, nil, 0)
	var result strings.Builder
	end := 0
	for {
		position, kind, literal := lexer.Scan()
		if kind == token.EOF {
			break
		}
		if isInsertedGoSemicolon(kind, literal) {
			continue
		}
		start := file.Offset(position)
		if start > end {
			_ = result.WriteByte(' ')
		}
		text, length := singleLineGoToken(kind, literal)
		end = start + length
		_, _ = result.WriteString(text)
	}
	return result.String()
}

func isInsertedGoSemicolon(kind token.Token, literal string) bool {
	return kind == token.SEMICOLON && literal == "\n"
}

func singleLineGoToken(kind token.Token, literal string) (string, int) {
	if literal == "" {
		text := kind.String()
		return text, len(text)
	}
	if kind == token.STRING && strings.HasPrefix(literal, "`") {
		value, _ := strconv.Unquote(literal)
		return strconv.Quote(value), len(literal)
	}
	return literal, len(literal)
}
