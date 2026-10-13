package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestExtractsDeclarationsWithoutImplementation(t *testing.T) {
	root := t.TempDir()
	source := `package sample
type Reader interface { Read(buffer []byte) (count int, err error) }
type Box[T any] struct { value T }
func exported[T ~int](value T) T { panic("SECRET IMPLEMENTATION") }
func (box *Box[T]) private(values ...T) (result T) { return box.value }
`
	err := os.WriteFile(
		filepath.Join(root, "sample.go"), []byte(source), 0600,
	)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(
		filepath.Join(root, "sample_test.go"), []byte("invalid test"), 0600,
	)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := writeGoSignatures(&output, root); err != nil {
		t.Fatal(err)
	}
	want := "file: sample.go\n. (package sample) interface Reader: " +
		"func Read(buffer []byte) (count int, err error)\n" +
		". (package sample): func exported[T ~int](value T) T\n" +
		". (package sample): " +
		"func (box *Box[T]) private(values ...T) (result T)\n"
	if output.String() != want {
		t.Fatalf("signatures = %q, want %q", output.String(), want)
	}
}

func TestInvalidGoSourceReturnsError(t *testing.T) {
	root := t.TempDir()
	err := os.WriteFile(
		filepath.Join(root, "invalid.go"), []byte("package"), 0600,
	)
	if err != nil {
		t.Fatal(err)
	}
	err = writeGoSignatures(&bytes.Buffer{}, root)
	if err == nil || !strings.Contains(err.Error(), "invalid.go") {
		t.Fatalf("expected parse error identifying invalid.go, got %v", err)
	}
}

func TestFileHeadersGroupSignaturesByRelativePath(t *testing.T) {
	root := t.TempDir()
	testutil.NoError(t, os.Mkdir(filepath.Join(root, "nested"), 0700))
	for _, path := range []string{"first.go", "nested/second.go"} {
		testutil.NoError(t, os.WriteFile(
			filepath.Join(root, filepath.FromSlash(path)),
			[]byte("package sample\nfunc Run() {}\nfunc Stop() {}"),
			0600,
		))
	}
	var output bytes.Buffer
	testutil.NoError(t, writeGoSignatures(&output, root))
	want := "file: first.go\n" +
		". (package sample): func Run()\n" +
		". (package sample): func Stop()\n" +
		"file: nested/second.go\n" +
		"nested (package sample): func Run()\n" +
		"nested (package sample): func Stop()\n"
	testutil.RequireEqual(t, output.String(), want, "file groups")
}

func TestExtractsNestedAndLocalInterfaceMethods(t *testing.T) {
	root := t.TempDir()
	source := `package sample
// Documentation must not appear in the inventory.
type Service interface {
 Execute(input interface { Read(data []byte) error }) error
}
func Start() {
 type Local interface { Close() error }
 var sink interface { Write(data []byte) (int, error) }
 _ = sink
}
`
	testutil.NoError(t, os.WriteFile(
		filepath.Join(root, "sample.go"), []byte(source), 0600,
	))
	var output bytes.Buffer
	testutil.NoError(t, writeGoSignatures(&output, root))
	got := output.String()
	for _, signature := range []string{
		"interface Service: func Execute(input interface",
		"interface anonymous@sample.go:4:16: func Read(data []byte) error",
		"func Start()",
		"interface Local: func Close() error",
		"interface anonymous@sample.go:8:11: " +
			"func Write(data []byte) (int, error)",
	} {
		if !strings.Contains(got, signature) {
			t.Fatalf("missing %q in signatures:\n%s", signature, got)
		}
	}
	if strings.Contains(got,
		"Documentation") || strings.Contains(got,
		"var sink") {
		t.Fatalf("documentation or implementation leaked: %s", got)
	}
	testutil.RequireEqual(t,
		strings.Count(got,
			"\n"),
		6,
		"file header and one line per signature")
}

func TestSingleLineSignaturePreservesLiteralWhitespace(t *testing.T) {
	source := "func describe(\n value struct { " +
		"Label string `label:\"two  words\"` },\n)"
	got := singleLineGoSignature([]byte(source))
	want := `func describe( value struct { ` +
		`Label string "label:\"two  words\"" }, )`
	if got != want {
		t.Fatalf("signature = %q, want %q", got, want)
	}
}

func TestSkipsDependencyAndTestDirectories(t *testing.T) {
	for _, directory := range []string{
		".git", "vendor", "node_modules", "testdata", "testutil",
	} {
		t.Run(directory, func(t *testing.T) {
			root := t.TempDir()
			excluded := filepath.Join(root, directory)
			testutil.NoError(t, os.Mkdir(excluded, 0700))
			err := os.WriteFile(
				filepath.Join(excluded, "invalid.go"), []byte("invalid"), 0600,
			)
			testutil.NoError(t, err)
			var output bytes.Buffer
			testutil.NoError(t, writeGoSignatures(&output, root))
			if output.Len() != 0 {
				t.Fatalf("unexpected signatures: %s", &output)
			}
		})
	}
}

func TestSignatureExtractionCLISelectsRootAndOutput(t *testing.T) {
	root := t.TempDir()
	err := os.WriteFile(
		filepath.Join(root, "sample.go"),
		[]byte("package sample\nfunc Run() {}"),
		0600,
	)
	testutil.NoError(t, err)
	var stdout bytes.Buffer
	testutil.NoError(t, extractProductionSignaturesCLI(
		[]string{"-root", root}, &stdout,
	))
	want := "file: sample.go\n. (package sample): func Run()\n"
	testutil.RequireEqual(t, stdout.String(), want, "stdout signatures")
	output := filepath.Join(root, "signatures.txt")
	stdout.Reset()
	testutil.NoError(t, extractProductionSignaturesCLI(
		[]string{"-root", root, "-output", output}, &stdout,
	))
	contents, err := os.ReadFile(output)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, string(contents), want, "file signatures")
	testutil.RequireEqual(t, stdout.Len(), 0, "file output uses no stdout")
}
