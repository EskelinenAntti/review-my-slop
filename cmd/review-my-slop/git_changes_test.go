package main

import (
	bytes "bytes"
	cmp "cmp"
	context "context"
	errors "errors"
	fmt "fmt"
	os "os"
	exec "os/exec"
	filepath "path/filepath"
	reflect "reflect"
	slices "slices"
	strings "strings"
	testing "testing"

	diff "github.com/sourcegraph/go-diff/diff"
)

func TestPatchGetIncludesUnstagedAndUntrackedButNotStagedOnly(t *testing.T) {
	repo := func() string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, err)
		var args []string = []string{"init", "-q"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		{
			var args []string = []string{"config", "user.email", "test@example.com"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		{
			var args []string = []string{"config", "user.name", "Test"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		return repo
	}()
	t.Helper()
	path := filepath.Join(repo, "modified.go")
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, os.MkdirAll(filepath.Dir(path), 0o755))
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, os.WriteFile(path, []byte("package main\n\nfunc value() int { return 1 }\n"), 0o644))
	{
		t.Helper()
		path := filepath.Join(repo, "staged.txt")
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.MkdirAll(filepath.Dir(path), 0o755))
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.WriteFile(path, []byte("before\n"), 0o644))
	}
	var args []string = []string{"add", "."}
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	{
		var args []string = []string{"commit", "-m", "base"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	{
		t.Helper()
		path := filepath.Join(repo, "modified.go")
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.MkdirAll(filepath.Dir(path), 0o755))
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.WriteFile(path, []byte("package main\n\nfunc value() int { return 2 }\n"), 0o644))
	}
	{
		t.Helper()
		path := filepath.Join(repo, "staged.txt")
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.MkdirAll(filepath.Dir(path), 0o755))
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.WriteFile(path, []byte("after\n"), 0o644))
	}
	{
		var args []string = []string{"add", "staged.txt"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		t.Helper()
		path := filepath.Join(repo, "new.py")
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.MkdirAll(filepath.Dir(path), 0o755))
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.WriteFile(path, []byte("def hello():\n    return 'world'\n"), 0o644))
	}

	t.Chdir(repo)
	got, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
		directory, err := os.Getwd()
		if err != nil {
			return patchPatch{}, fmt.Errorf("resolve working directory: %w", err)
		}
		{
			if kind != patchUnstaged && kind != patchBranch {
				return patchPatch{}, fmt.Errorf("unknown patch kind %d", kind)
			}
			if err := ctx.Err(); err != nil {
				return patchPatch{}, err
			}
			root, err := func() (string, error) {
				rootBytes, err := patchRunner(patchExecRunner{}).Run(ctx, directory, "rev-parse", "--show-toplevel")
				if err != nil {
					return "", err
				}
				root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
				if err != nil {
					return "", fmt.Errorf("resolve repository root: %w", err)
				}
				return root, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			branch, err := func() (string, error) {
				out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
				if err == nil {
					return strings.TrimSpace(string(out)), nil
				}
				for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
					_, err := patchRunner(patchExecRunner{}).Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
					if ctx.Err() != nil {
						return "", ctx.Err()
					}
					if err == nil {
						return candidate, nil
					}
				}
				return "", nil
			}()
			if err != nil {
				return patchPatch{}, fmt.Errorf("discover default branch: %w", err)
			}
			revisions, readOld, err := func() ([]string, patchSourceReader, error) {
				if kind != patchBranch {
					return nil, func(ctx context.Context, run patchRunner, root, path string) string {
						if path == "" || path == "/dev/null" {
							return ""
						}
						out, err := run.Run(ctx, root, "show", ":"+path)
						if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
							return ""
						}
						var result strings.Builder
						for _, r := range string(out) {
							switch {
							case r == '\n' || r == '\t':
								_, _ = result.WriteRune(r)
							case r == '\r':
								_, _ = result.WriteString(`\r`)
							case r < 0x20 || r == 0x7f:
								fmt.Fprintf(&result, `\x%02x`, r)
							default:
								_, _ = result.WriteRune(r)
							}
						}
						return result.String()
					}, nil
				}
				if branch == "" {
					return nil, nil, errors.New("default branch is unavailable")
				}
				baseBytes, err := patchRunner(patchExecRunner{}).Run(ctx, root, "merge-base", branch, "HEAD")
				if err != nil {
					return nil, nil, fmt.Errorf("find branch point with %s: %w", branch, err)
				}
				base := strings.TrimSpace(string(baseBytes))
				readOld := func(ctx context.Context, run patchRunner, root, path string) string {
					if path == "" || path == "/dev/null" {
						return ""
					}
					out, err := run.Run(ctx, root, "show", base+":"+path)
					if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
						return ""
					}
					var result strings.Builder
					for _, r := range string(out) {
						switch {
						case r == '\n' || r == '\t':
							_, _ = result.WriteRune(r)
						case r == '\r':
							_, _ = result.WriteString(`\r`)
						case r < 0x20 || r == 0x7f:
							fmt.Fprintf(&result, `\x%02x`, r)
						default:
							_, _ = result.WriteRune(r)
						}
					}
					return result.String()
				}
				return []string{base}, readOld, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			raw, err := func() ([]byte, error) {
				args := []string{"-c", "core.quotepath=false", "-c", "diff.external=", "--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--unified=3"}
				args = append(args, revisions...)
				args = append(args, "--")
				return patchRunner(patchExecRunner{}).Run(ctx, root, args...)
			}()
			if err != nil {
				return patchPatch{}, err
			}
			{
				files, err := func() ([]patchFile, error) {
					if len(bytes.TrimSpace(raw)) == 0 {
						return nil, nil
					}
					parsed, err := diff.ParseMultiFileDiff(raw)
					if err != nil {
						return nil, fmt.Errorf("parse git diff: %w", err)
					}
					files := make([]patchFile, 0, len(parsed))
					for _, fd := range parsed {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, err := func() (patchFile, error) {
							oldPath := func() string {
								var path string = fd.OrigName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							newPath := func() string {
								var path string = fd.NewName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							display := newPath
							if display == "" || display == "/dev/null" {
								display = oldPath
							}
							file := patchFile{OldPath: oldPath, NewPath: newPath, DisplayPath: func() string {
								var value string = display
								return strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range value {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`)
							}(), Metadata: func() []string {
								var values []string = fd.Extended
								result := make([]string, len(values))
								for index, value := range values {
									result[index] = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range value {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
								}
								return result
							}()}
							file.OldSource = readOld(ctx, patchExecRunner{}, root, oldPath)
							file.NewSource = func() string {
								if newPath == "" || newPath == "/dev/null" {
									return ""
								}
								full := filepath.Join(root, filepath.FromSlash(newPath))
								info, err := os.Lstat(full)
								if err != nil || !info.Mode().IsRegular() || info.Size() > patchMaxFileBytes {
									return ""
								}
								out, err := os.ReadFile(full)
								if err != nil || bytes.IndexByte(out, 0) >= 0 {
									return ""
								}
								var result strings.Builder
								for _, r := range string(out) {
									switch {
									case r == '\n' || r == '\t':
										_, _ = result.WriteRune(r)
									case r == '\r':
										_, _ = result.WriteString(`\r`)
									case r < 0x20 || r == 0x7f:
										fmt.Fprintf(&result, `\x%02x`, r)
									default:
										_, _ = result.WriteRune(r)
									}
								}
								return result.String()
							}()
							for _, h := range fd.Hunks {
								lines, err := func() ([]patchLine, error) {
									var (
										oldLine, newLine int32  = h.OrigStartLine, h.NewStartLine
										body             []byte = h.Body
									)
									rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
									if len(body) == 0 {
										return []patchLine{}, nil
									}
									lines := make([]patchLine, 0, len(rawLines))
									for _, raw := range rawLines {
										if len(raw) == 0 {
											return nil, errors.New("malformed empty diff line")
										}
										text := func() string {
											var value string = string(raw[1:])
											return strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range value {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`)
										}()
										switch raw[0] {
										case ' ':
											lines = append(lines, patchLine{Kind: patchContext, Text: text, OldNumber: patchLineNumber(oldLine), NewNumber: patchLineNumber(newLine)})
											oldLine++
											newLine++
										case '+':
											lines = append(lines, patchLine{Kind: patchAddition, Text: text, NewNumber: patchLineNumber(newLine)})
											newLine++
										case '-':
											lines = append(lines, patchLine{Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(oldLine)})
											oldLine++
										case '\\':
										default:
											return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
										}
									}
									return lines, nil
								}()
								if err != nil {
									return patchFile{}, fmt.Errorf("%s: %w", display, err)
								}
								file.Hunks = append(file.Hunks, patchHunk{Header: func() string {
									header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
									if h.Section != "" {
										header += " " + h.Section
									}
									return header
								}(), Lines: lines})
							}
							return file, nil
						}()
						if err != nil {
							return nil, err
						}
						files = append(files, file)
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				untracked, err := func() ([]patchFile, error) {
					out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
					if err != nil {
						return nil, err
					}
					var files []patchFile
					for _, path := range func() []string {
						var paths []string
						for path := range bytes.SplitSeq(out, []byte{0}) {
							if len(path) > 0 {
								paths = append(paths, string(path))
							}
						}
						return paths
					}() {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, include, err := func() (patchFile, bool, error) {
							full := filepath.Join(root, filepath.FromSlash(path))
							info, err := os.Lstat(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
							}
							if info.Mode()&os.ModeSymlink != 0 {
								target, err := os.Readlink(full)
								if err != nil {
									return patchFile{}, false, fmt.Errorf("read symlink %q: %w", path, err)
								}
								return func() patchFile {
									var content string = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range target {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
									sourceLines := func() []string {
										var content string = content
										content = strings.TrimSuffix(content, "\n")
										if content == "" {
											return nil
										}
										return strings.Split(content, "\n")
									}()
									lines := make([]patchLine, 0, len(sourceLines))
									for i, line := range sourceLines {
										lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
									}
									return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range path {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
								}(), true, nil
							}
							if !info.Mode().IsRegular() {
								return patchFile{}, false, nil
							}
							if info.Size() > patchMaxFileBytes {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked file", "content omitted: file exceeds 2 MiB"}}, true, nil
							}
							content, err := os.ReadFile(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("read untracked %q: %w", path, err)
							}
							if bytes.IndexByte(content, 0) >= 0 {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked binary file"}}, true, nil
							}
							return func() patchFile {
								var content string = func() string {
									var result strings.Builder
									for _, r := range string(content) {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}()
								sourceLines := func() []string {
									var content string = content
									content = strings.TrimSuffix(content, "\n")
									if content == "" {
										return nil
									}
									return strings.Split(content, "\n")
								}()
								lines := make([]patchLine, 0, len(sourceLines))
								for i, line := range sourceLines {
									lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
								}
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
							}(), true, nil
						}()
						if err != nil {
							return nil, err
						}
						if include {
							files = append(files, file)
						}
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				files = append(files, untracked...)
				slices.SortStableFunc(files, func(a, b patchFile) int { return cmp.Compare(a.DisplayPath, b.DisplayPath) })
				if err := ctx.Err(); err != nil {
					return patchPatch{}, err
				}
				return patchPatch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
			}
		}
	})(context.Background(), patchUnstaged)
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got.Root, repo, "root = %q, kind = %v", got.Root,
		got.Kind,
	)
	(func(t *testing.T, got, want patchKind, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got.Kind, patchUnstaged, "root = %q, kind = %v",
		got.Root, got.Kind,
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(got.Files), 2, "files = %d, want 2: %#v",
		len(got.Files), got.Files,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got.Files[0].DisplayPath, "modified.go",
		"unexpected files: %q, %q", got.Files[0].DisplayPath,
		got.Files[1].DisplayPath,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got.Files[1].DisplayPath, "new.py",
		"unexpected files: %q, %q", got.Files[0].DisplayPath,
		got.Files[1].DisplayPath,
	)

	modified := got.Files[0]
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() bool {
		for _, hunk := range modified.Hunks {
			for _, line := range hunk.Lines {
				if line.Kind == patchDeletion && strings.Contains(line.Text, "return 1") {
					return true
				}
			}
		}
		return false
	}(), true,
		"modified file lacks expected lines: %#v", modified.Hunks,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		func() bool {
			for _, hunk := range modified.Hunks {
				for _, line := range hunk.Lines {
					if line.Kind == patchAddition && strings.Contains(line.Text, "return 2") {
						return true
					}
				}
			}
			return false
		}(), true, "modified file lacks expected lines: %#v", modified.Hunks)

	untracked := got.Files[1]
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, untracked.OldSource, "",
		"unexpected untracked sources: %#v", untracked,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(untracked.NewSource, "def hello"),
		true, "unexpected untracked sources: %#v", untracked,
	)

	for _, hunk := range untracked.Hunks {
		for _, line := range hunk.Lines {
			(func(t *testing.T, got, want patchLineKind, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, line.Kind, patchAddition,
				"unexpected untracked line: %#v", line,
			)
			(func(t *testing.T, got, want patchLineNumber, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, line.OldNumber, 0,
				"unexpected untracked line: %#v", line,
			)
			(func(t *testing.T, got, want bool, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, line.NewNumber != 0, true,
				"unexpected untracked line: %#v", line,
			)
		}
	}
}

func TestPatchAddedFileKeepsRawAndDisplayPathsSeparate(t *testing.T) {
	file := func() patchFile {
		var path, content string = "odd\nname.go", "package main\n"
		sourceLines := func() []string {
			var content string = content
			content = strings.TrimSuffix(content, "\n")
			if content == "" {
				return nil
			}
			return strings.Split(content, "\n")
		}()
		lines := make([]patchLine, 0, len(sourceLines))
		for i, line := range sourceLines {
			lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
		}
		return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
			var result strings.Builder
			for _, r := range path {
				switch {
				case r == '\n' || r == '\t':
					_, _ = result.WriteRune(r)
				case r == '\r':
					_, _ = result.WriteString(`\r`)
				case r < 0x20 || r == 0x7f:
					fmt.Fprintf(&result, `\x%02x`, r)
				default:
					_, _ = result.WriteRune(r)
				}
			}
			return result.String()
		}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
	}()
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, file.NewPath, "odd\nname.go", "raw path = %q",
		file.NewPath,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, file.DisplayPath, `odd\nname.go`,
		"display path = %q", file.DisplayPath,
	)
}

func TestPatchGetFromSubdirectoryKeepsRawUntrackedPaths(t *testing.T) {
	repo := func() string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, err)
		var args []string = []string{"init", "-q"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		{
			var args []string = []string{"config", "user.email", "test@example.com"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		{
			var args []string = []string{"config", "user.name", "Test"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		return repo
	}()
	t.Helper()
	path := filepath.Join(repo, "nested/odd\nname.go")
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, os.MkdirAll(filepath.Dir(path), 0o755))
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, os.WriteFile(path, []byte("package main\n"), 0o644))
	t.Chdir(filepath.Join(repo, "nested"))

	got, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
		directory, err := os.Getwd()
		if err != nil {
			return patchPatch{}, fmt.Errorf("resolve working directory: %w", err)
		}
		{
			if kind != patchUnstaged && kind != patchBranch {
				return patchPatch{}, fmt.Errorf("unknown patch kind %d", kind)
			}
			if err := ctx.Err(); err != nil {
				return patchPatch{}, err
			}
			root, err := func() (string, error) {
				rootBytes, err := patchRunner(patchExecRunner{}).Run(ctx, directory, "rev-parse", "--show-toplevel")
				if err != nil {
					return "", err
				}
				root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
				if err != nil {
					return "", fmt.Errorf("resolve repository root: %w", err)
				}
				return root, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			branch, err := func() (string, error) {
				out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
				if err == nil {
					return strings.TrimSpace(string(out)), nil
				}
				for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
					_, err := patchRunner(patchExecRunner{}).Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
					if ctx.Err() != nil {
						return "", ctx.Err()
					}
					if err == nil {
						return candidate, nil
					}
				}
				return "", nil
			}()
			if err != nil {
				return patchPatch{}, fmt.Errorf("discover default branch: %w", err)
			}
			revisions, readOld, err := func() ([]string, patchSourceReader, error) {
				if kind != patchBranch {
					return nil, func(ctx context.Context, run patchRunner, root, path string) string {
						if path == "" || path == "/dev/null" {
							return ""
						}
						out, err := run.Run(ctx, root, "show", ":"+path)
						if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
							return ""
						}
						var result strings.Builder
						for _, r := range string(out) {
							switch {
							case r == '\n' || r == '\t':
								_, _ = result.WriteRune(r)
							case r == '\r':
								_, _ = result.WriteString(`\r`)
							case r < 0x20 || r == 0x7f:
								fmt.Fprintf(&result, `\x%02x`, r)
							default:
								_, _ = result.WriteRune(r)
							}
						}
						return result.String()
					}, nil
				}
				if branch == "" {
					return nil, nil, errors.New("default branch is unavailable")
				}
				baseBytes, err := patchRunner(patchExecRunner{}).Run(ctx, root, "merge-base", branch, "HEAD")
				if err != nil {
					return nil, nil, fmt.Errorf("find branch point with %s: %w", branch, err)
				}
				base := strings.TrimSpace(string(baseBytes))
				readOld := func(ctx context.Context, run patchRunner, root, path string) string {
					if path == "" || path == "/dev/null" {
						return ""
					}
					out, err := run.Run(ctx, root, "show", base+":"+path)
					if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
						return ""
					}
					var result strings.Builder
					for _, r := range string(out) {
						switch {
						case r == '\n' || r == '\t':
							_, _ = result.WriteRune(r)
						case r == '\r':
							_, _ = result.WriteString(`\r`)
						case r < 0x20 || r == 0x7f:
							fmt.Fprintf(&result, `\x%02x`, r)
						default:
							_, _ = result.WriteRune(r)
						}
					}
					return result.String()
				}
				return []string{base}, readOld, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			raw, err := func() ([]byte, error) {
				args := []string{"-c", "core.quotepath=false", "-c", "diff.external=", "--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--unified=3"}
				args = append(args, revisions...)
				args = append(args, "--")
				return patchRunner(patchExecRunner{}).Run(ctx, root, args...)
			}()
			if err != nil {
				return patchPatch{}, err
			}
			{
				files, err := func() ([]patchFile, error) {
					if len(bytes.TrimSpace(raw)) == 0 {
						return nil, nil
					}
					parsed, err := diff.ParseMultiFileDiff(raw)
					if err != nil {
						return nil, fmt.Errorf("parse git diff: %w", err)
					}
					files := make([]patchFile, 0, len(parsed))
					for _, fd := range parsed {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, err := func() (patchFile, error) {
							oldPath := func() string {
								var path string = fd.OrigName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							newPath := func() string {
								var path string = fd.NewName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							display := newPath
							if display == "" || display == "/dev/null" {
								display = oldPath
							}
							file := patchFile{OldPath: oldPath, NewPath: newPath, DisplayPath: func() string {
								var value string = display
								return strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range value {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`)
							}(), Metadata: func() []string {
								var values []string = fd.Extended
								result := make([]string, len(values))
								for index, value := range values {
									result[index] = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range value {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
								}
								return result
							}()}
							file.OldSource = readOld(ctx, patchExecRunner{}, root, oldPath)
							file.NewSource = func() string {
								if newPath == "" || newPath == "/dev/null" {
									return ""
								}
								full := filepath.Join(root, filepath.FromSlash(newPath))
								info, err := os.Lstat(full)
								if err != nil || !info.Mode().IsRegular() || info.Size() > patchMaxFileBytes {
									return ""
								}
								out, err := os.ReadFile(full)
								if err != nil || bytes.IndexByte(out, 0) >= 0 {
									return ""
								}
								var result strings.Builder
								for _, r := range string(out) {
									switch {
									case r == '\n' || r == '\t':
										_, _ = result.WriteRune(r)
									case r == '\r':
										_, _ = result.WriteString(`\r`)
									case r < 0x20 || r == 0x7f:
										fmt.Fprintf(&result, `\x%02x`, r)
									default:
										_, _ = result.WriteRune(r)
									}
								}
								return result.String()
							}()
							for _, h := range fd.Hunks {
								lines, err := func() ([]patchLine, error) {
									var (
										oldLine, newLine int32  = h.OrigStartLine, h.NewStartLine
										body             []byte = h.Body
									)
									rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
									if len(body) == 0 {
										return []patchLine{}, nil
									}
									lines := make([]patchLine, 0, len(rawLines))
									for _, raw := range rawLines {
										if len(raw) == 0 {
											return nil, errors.New("malformed empty diff line")
										}
										text := func() string {
											var value string = string(raw[1:])
											return strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range value {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`)
										}()
										switch raw[0] {
										case ' ':
											lines = append(lines, patchLine{Kind: patchContext, Text: text, OldNumber: patchLineNumber(oldLine), NewNumber: patchLineNumber(newLine)})
											oldLine++
											newLine++
										case '+':
											lines = append(lines, patchLine{Kind: patchAddition, Text: text, NewNumber: patchLineNumber(newLine)})
											newLine++
										case '-':
											lines = append(lines, patchLine{Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(oldLine)})
											oldLine++
										case '\\':
										default:
											return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
										}
									}
									return lines, nil
								}()
								if err != nil {
									return patchFile{}, fmt.Errorf("%s: %w", display, err)
								}
								file.Hunks = append(file.Hunks, patchHunk{Header: func() string {
									header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
									if h.Section != "" {
										header += " " + h.Section
									}
									return header
								}(), Lines: lines})
							}
							return file, nil
						}()
						if err != nil {
							return nil, err
						}
						files = append(files, file)
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				untracked, err := func() ([]patchFile, error) {
					out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
					if err != nil {
						return nil, err
					}
					var files []patchFile
					for _, path := range func() []string {
						var paths []string
						for path := range bytes.SplitSeq(out, []byte{0}) {
							if len(path) > 0 {
								paths = append(paths, string(path))
							}
						}
						return paths
					}() {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, include, err := func() (patchFile, bool, error) {
							full := filepath.Join(root, filepath.FromSlash(path))
							info, err := os.Lstat(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
							}
							if info.Mode()&os.ModeSymlink != 0 {
								target, err := os.Readlink(full)
								if err != nil {
									return patchFile{}, false, fmt.Errorf("read symlink %q: %w", path, err)
								}
								return func() patchFile {
									var content string = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range target {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
									sourceLines := func() []string {
										var content string = content
										content = strings.TrimSuffix(content, "\n")
										if content == "" {
											return nil
										}
										return strings.Split(content, "\n")
									}()
									lines := make([]patchLine, 0, len(sourceLines))
									for i, line := range sourceLines {
										lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
									}
									return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range path {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
								}(), true, nil
							}
							if !info.Mode().IsRegular() {
								return patchFile{}, false, nil
							}
							if info.Size() > patchMaxFileBytes {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked file", "content omitted: file exceeds 2 MiB"}}, true, nil
							}
							content, err := os.ReadFile(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("read untracked %q: %w", path, err)
							}
							if bytes.IndexByte(content, 0) >= 0 {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked binary file"}}, true, nil
							}
							return func() patchFile {
								var content string = func() string {
									var result strings.Builder
									for _, r := range string(content) {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}()
								sourceLines := func() []string {
									var content string = content
									content = strings.TrimSuffix(content, "\n")
									if content == "" {
										return nil
									}
									return strings.Split(content, "\n")
								}()
								lines := make([]patchLine, 0, len(sourceLines))
								for i, line := range sourceLines {
									lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
								}
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
							}(), true, nil
						}()
						if err != nil {
							return nil, err
						}
						if include {
							files = append(files, file)
						}
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				files = append(files, untracked...)
				slices.SortStableFunc(files, func(a, b patchFile) int { return cmp.Compare(a.DisplayPath, b.DisplayPath) })
				if err := ctx.Err(); err != nil {
					return patchPatch{}, err
				}
				return patchPatch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
			}
		}
	})(context.Background(), patchUnstaged)
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got.Root, repo, "patch = %#v", got)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(got.Files), 1, "patch = %#v", got)

	file := got.Files[0]
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, file.OldPath, "", "file paths = %#v", file)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, file.NewPath, "nested/odd\nname.go",
		"file paths = %#v", file,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, file.DisplayPath, `nested/odd\nname.go`,
		"file paths = %#v", file,
	)
}

func TestPatchGetReportsUnavailableBranchAndInvalidKind(t *testing.T) {
	t.Chdir(func() string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, err)
		var args []string = []string{"init", "-q"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		{
			var args []string = []string{"config", "user.email", "test@example.com"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		{
			var args []string = []string{"config", "user.name", "Test"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		return repo
	}())
	local, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
		directory, err := os.Getwd()
		if err != nil {
			return patchPatch{}, fmt.Errorf("resolve working directory: %w", err)
		}
		{
			if kind != patchUnstaged && kind != patchBranch {
				return patchPatch{}, fmt.Errorf("unknown patch kind %d", kind)
			}
			if err := ctx.Err(); err != nil {
				return patchPatch{}, err
			}
			root, err := func() (string, error) {
				rootBytes, err := patchRunner(patchExecRunner{}).Run(ctx, directory, "rev-parse", "--show-toplevel")
				if err != nil {
					return "", err
				}
				root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
				if err != nil {
					return "", fmt.Errorf("resolve repository root: %w", err)
				}
				return root, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			branch, err := func() (string, error) {
				out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
				if err == nil {
					return strings.TrimSpace(string(out)), nil
				}
				for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
					_, err := patchRunner(patchExecRunner{}).Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
					if ctx.Err() != nil {
						return "", ctx.Err()
					}
					if err == nil {
						return candidate, nil
					}
				}
				return "", nil
			}()
			if err != nil {
				return patchPatch{}, fmt.Errorf("discover default branch: %w", err)
			}
			revisions, readOld, err := func() ([]string, patchSourceReader, error) {
				if kind != patchBranch {
					return nil, func(ctx context.Context, run patchRunner, root, path string) string {
						if path == "" || path == "/dev/null" {
							return ""
						}
						out, err := run.Run(ctx, root, "show", ":"+path)
						if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
							return ""
						}
						var result strings.Builder
						for _, r := range string(out) {
							switch {
							case r == '\n' || r == '\t':
								_, _ = result.WriteRune(r)
							case r == '\r':
								_, _ = result.WriteString(`\r`)
							case r < 0x20 || r == 0x7f:
								fmt.Fprintf(&result, `\x%02x`, r)
							default:
								_, _ = result.WriteRune(r)
							}
						}
						return result.String()
					}, nil
				}
				if branch == "" {
					return nil, nil, errors.New("default branch is unavailable")
				}
				baseBytes, err := patchRunner(patchExecRunner{}).Run(ctx, root, "merge-base", branch, "HEAD")
				if err != nil {
					return nil, nil, fmt.Errorf("find branch point with %s: %w", branch, err)
				}
				base := strings.TrimSpace(string(baseBytes))
				readOld := func(ctx context.Context, run patchRunner, root, path string) string {
					if path == "" || path == "/dev/null" {
						return ""
					}
					out, err := run.Run(ctx, root, "show", base+":"+path)
					if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
						return ""
					}
					var result strings.Builder
					for _, r := range string(out) {
						switch {
						case r == '\n' || r == '\t':
							_, _ = result.WriteRune(r)
						case r == '\r':
							_, _ = result.WriteString(`\r`)
						case r < 0x20 || r == 0x7f:
							fmt.Fprintf(&result, `\x%02x`, r)
						default:
							_, _ = result.WriteRune(r)
						}
					}
					return result.String()
				}
				return []string{base}, readOld, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			raw, err := func() ([]byte, error) {
				args := []string{"-c", "core.quotepath=false", "-c", "diff.external=", "--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--unified=3"}
				args = append(args, revisions...)
				args = append(args, "--")
				return patchRunner(patchExecRunner{}).Run(ctx, root, args...)
			}()
			if err != nil {
				return patchPatch{}, err
			}
			{
				files, err := func() ([]patchFile, error) {
					if len(bytes.TrimSpace(raw)) == 0 {
						return nil, nil
					}
					parsed, err := diff.ParseMultiFileDiff(raw)
					if err != nil {
						return nil, fmt.Errorf("parse git diff: %w", err)
					}
					files := make([]patchFile, 0, len(parsed))
					for _, fd := range parsed {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, err := func() (patchFile, error) {
							oldPath := func() string {
								var path string = fd.OrigName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							newPath := func() string {
								var path string = fd.NewName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							display := newPath
							if display == "" || display == "/dev/null" {
								display = oldPath
							}
							file := patchFile{OldPath: oldPath, NewPath: newPath, DisplayPath: func() string {
								var value string = display
								return strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range value {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`)
							}(), Metadata: func() []string {
								var values []string = fd.Extended
								result := make([]string, len(values))
								for index, value := range values {
									result[index] = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range value {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
								}
								return result
							}()}
							file.OldSource = readOld(ctx, patchExecRunner{}, root, oldPath)
							file.NewSource = func() string {
								if newPath == "" || newPath == "/dev/null" {
									return ""
								}
								full := filepath.Join(root, filepath.FromSlash(newPath))
								info, err := os.Lstat(full)
								if err != nil || !info.Mode().IsRegular() || info.Size() > patchMaxFileBytes {
									return ""
								}
								out, err := os.ReadFile(full)
								if err != nil || bytes.IndexByte(out, 0) >= 0 {
									return ""
								}
								var result strings.Builder
								for _, r := range string(out) {
									switch {
									case r == '\n' || r == '\t':
										_, _ = result.WriteRune(r)
									case r == '\r':
										_, _ = result.WriteString(`\r`)
									case r < 0x20 || r == 0x7f:
										fmt.Fprintf(&result, `\x%02x`, r)
									default:
										_, _ = result.WriteRune(r)
									}
								}
								return result.String()
							}()
							for _, h := range fd.Hunks {
								lines, err := func() ([]patchLine, error) {
									var (
										oldLine, newLine int32  = h.OrigStartLine, h.NewStartLine
										body             []byte = h.Body
									)
									rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
									if len(body) == 0 {
										return []patchLine{}, nil
									}
									lines := make([]patchLine, 0, len(rawLines))
									for _, raw := range rawLines {
										if len(raw) == 0 {
											return nil, errors.New("malformed empty diff line")
										}
										text := func() string {
											var value string = string(raw[1:])
											return strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range value {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`)
										}()
										switch raw[0] {
										case ' ':
											lines = append(lines, patchLine{Kind: patchContext, Text: text, OldNumber: patchLineNumber(oldLine), NewNumber: patchLineNumber(newLine)})
											oldLine++
											newLine++
										case '+':
											lines = append(lines, patchLine{Kind: patchAddition, Text: text, NewNumber: patchLineNumber(newLine)})
											newLine++
										case '-':
											lines = append(lines, patchLine{Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(oldLine)})
											oldLine++
										case '\\':
										default:
											return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
										}
									}
									return lines, nil
								}()
								if err != nil {
									return patchFile{}, fmt.Errorf("%s: %w", display, err)
								}
								file.Hunks = append(file.Hunks, patchHunk{Header: func() string {
									header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
									if h.Section != "" {
										header += " " + h.Section
									}
									return header
								}(), Lines: lines})
							}
							return file, nil
						}()
						if err != nil {
							return nil, err
						}
						files = append(files, file)
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				untracked, err := func() ([]patchFile, error) {
					out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
					if err != nil {
						return nil, err
					}
					var files []patchFile
					for _, path := range func() []string {
						var paths []string
						for path := range bytes.SplitSeq(out, []byte{0}) {
							if len(path) > 0 {
								paths = append(paths, string(path))
							}
						}
						return paths
					}() {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, include, err := func() (patchFile, bool, error) {
							full := filepath.Join(root, filepath.FromSlash(path))
							info, err := os.Lstat(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
							}
							if info.Mode()&os.ModeSymlink != 0 {
								target, err := os.Readlink(full)
								if err != nil {
									return patchFile{}, false, fmt.Errorf("read symlink %q: %w", path, err)
								}
								return func() patchFile {
									var content string = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range target {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
									sourceLines := func() []string {
										var content string = content
										content = strings.TrimSuffix(content, "\n")
										if content == "" {
											return nil
										}
										return strings.Split(content, "\n")
									}()
									lines := make([]patchLine, 0, len(sourceLines))
									for i, line := range sourceLines {
										lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
									}
									return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range path {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
								}(), true, nil
							}
							if !info.Mode().IsRegular() {
								return patchFile{}, false, nil
							}
							if info.Size() > patchMaxFileBytes {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked file", "content omitted: file exceeds 2 MiB"}}, true, nil
							}
							content, err := os.ReadFile(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("read untracked %q: %w", path, err)
							}
							if bytes.IndexByte(content, 0) >= 0 {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked binary file"}}, true, nil
							}
							return func() patchFile {
								var content string = func() string {
									var result strings.Builder
									for _, r := range string(content) {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}()
								sourceLines := func() []string {
									var content string = content
									content = strings.TrimSuffix(content, "\n")
									if content == "" {
										return nil
									}
									return strings.Split(content, "\n")
								}()
								lines := make([]patchLine, 0, len(sourceLines))
								for i, line := range sourceLines {
									lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
								}
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
							}(), true, nil
						}()
						if err != nil {
							return nil, err
						}
						if include {
							files = append(files, file)
						}
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				files = append(files, untracked...)
				slices.SortStableFunc(files, func(a, b patchFile) int { return cmp.Compare(a.DisplayPath, b.DisplayPath) })
				if err := ctx.Err(); err != nil {
					return patchPatch{}, err
				}
				return patchPatch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
			}
		}
	})(context.Background(), patchUnstaged)
	(func(t *testing.T, got, want error, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, err, nil, "local patch = %#v, error = %v", local,
		err,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, local.Branch, "", "local patch = %#v, error = %v",
		local, err,
	)

	for _, test := range []struct {
		kind    patchKind
		message string
	}{
		{patchBranch, "default branch is unavailable"},
		{patchKind(255), "unknown patch kind"},
	} {
		got, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
			directory, err := os.Getwd()
			if err != nil {
				return patchPatch{}, fmt.Errorf("resolve working directory: %w", err)
			}
			{
				if kind != patchUnstaged && kind != patchBranch {
					return patchPatch{}, fmt.Errorf("unknown patch kind %d", kind)
				}
				if err := ctx.Err(); err != nil {
					return patchPatch{}, err
				}
				root, err := func() (string, error) {
					rootBytes, err := patchRunner(patchExecRunner{}).Run(ctx, directory, "rev-parse", "--show-toplevel")
					if err != nil {
						return "", err
					}
					root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
					if err != nil {
						return "", fmt.Errorf("resolve repository root: %w", err)
					}
					return root, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				branch, err := func() (string, error) {
					out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
					if ctx.Err() != nil {
						return "", ctx.Err()
					}
					if err == nil {
						return strings.TrimSpace(string(out)), nil
					}
					for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
						_, err := patchRunner(patchExecRunner{}).Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
						if ctx.Err() != nil {
							return "", ctx.Err()
						}
						if err == nil {
							return candidate, nil
						}
					}
					return "", nil
				}()
				if err != nil {
					return patchPatch{}, fmt.Errorf("discover default branch: %w", err)
				}
				revisions, readOld, err := func() ([]string, patchSourceReader, error) {
					if kind != patchBranch {
						return nil, func(ctx context.Context, run patchRunner, root, path string) string {
							if path == "" || path == "/dev/null" {
								return ""
							}
							out, err := run.Run(ctx, root, "show", ":"+path)
							if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
								return ""
							}
							var result strings.Builder
							for _, r := range string(out) {
								switch {
								case r == '\n' || r == '\t':
									_, _ = result.WriteRune(r)
								case r == '\r':
									_, _ = result.WriteString(`\r`)
								case r < 0x20 || r == 0x7f:
									fmt.Fprintf(&result, `\x%02x`, r)
								default:
									_, _ = result.WriteRune(r)
								}
							}
							return result.String()
						}, nil
					}
					if branch == "" {
						return nil, nil, errors.New("default branch is unavailable")
					}
					baseBytes, err := patchRunner(patchExecRunner{}).Run(ctx, root, "merge-base", branch, "HEAD")
					if err != nil {
						return nil, nil, fmt.Errorf("find branch point with %s: %w", branch, err)
					}
					base := strings.TrimSpace(string(baseBytes))
					readOld := func(ctx context.Context, run patchRunner, root, path string) string {
						if path == "" || path == "/dev/null" {
							return ""
						}
						out, err := run.Run(ctx, root, "show", base+":"+path)
						if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
							return ""
						}
						var result strings.Builder
						for _, r := range string(out) {
							switch {
							case r == '\n' || r == '\t':
								_, _ = result.WriteRune(r)
							case r == '\r':
								_, _ = result.WriteString(`\r`)
							case r < 0x20 || r == 0x7f:
								fmt.Fprintf(&result, `\x%02x`, r)
							default:
								_, _ = result.WriteRune(r)
							}
						}
						return result.String()
					}
					return []string{base}, readOld, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				raw, err := func() ([]byte, error) {
					args := []string{"-c", "core.quotepath=false", "-c", "diff.external=", "--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--unified=3"}
					args = append(args, revisions...)
					args = append(args, "--")
					return patchRunner(patchExecRunner{}).Run(ctx, root, args...)
				}()
				if err != nil {
					return patchPatch{}, err
				}
				{
					files, err := func() ([]patchFile, error) {
						if len(bytes.TrimSpace(raw)) == 0 {
							return nil, nil
						}
						parsed, err := diff.ParseMultiFileDiff(raw)
						if err != nil {
							return nil, fmt.Errorf("parse git diff: %w", err)
						}
						files := make([]patchFile, 0, len(parsed))
						for _, fd := range parsed {
							if err := ctx.Err(); err != nil {
								return nil, err
							}
							file, err := func() (patchFile, error) {
								oldPath := func() string {
									var path string = fd.OrigName
									path = strings.TrimSpace(path)
									path = strings.TrimPrefix(path, "a/")
									path = strings.TrimPrefix(path, "b/")
									if path == "/dev/null" {
										return ""
									}
									return path
								}()
								newPath := func() string {
									var path string = fd.NewName
									path = strings.TrimSpace(path)
									path = strings.TrimPrefix(path, "a/")
									path = strings.TrimPrefix(path, "b/")
									if path == "/dev/null" {
										return ""
									}
									return path
								}()
								display := newPath
								if display == "" || display == "/dev/null" {
									display = oldPath
								}
								file := patchFile{OldPath: oldPath, NewPath: newPath, DisplayPath: func() string {
									var value string = display
									return strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range value {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
								}(), Metadata: func() []string {
									var values []string = fd.Extended
									result := make([]string, len(values))
									for index, value := range values {
										result[index] = strings.ReplaceAll(func() string {
											var result strings.Builder
											for _, r := range value {
												switch {
												case r == '\n' || r == '\t':
													_, _ = result.WriteRune(r)
												case r == '\r':
													_, _ = result.WriteString(`\r`)
												case r < 0x20 || r == 0x7f:
													fmt.Fprintf(&result, `\x%02x`, r)
												default:
													_, _ = result.WriteRune(r)
												}
											}
											return result.String()
										}(), "\n", `\n`)
									}
									return result
								}()}
								file.OldSource = readOld(ctx, patchExecRunner{}, root, oldPath)
								file.NewSource = func() string {
									if newPath == "" || newPath == "/dev/null" {
										return ""
									}
									full := filepath.Join(root, filepath.FromSlash(newPath))
									info, err := os.Lstat(full)
									if err != nil || !info.Mode().IsRegular() || info.Size() > patchMaxFileBytes {
										return ""
									}
									out, err := os.ReadFile(full)
									if err != nil || bytes.IndexByte(out, 0) >= 0 {
										return ""
									}
									var result strings.Builder
									for _, r := range string(out) {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}()
								for _, h := range fd.Hunks {
									lines, err := func() ([]patchLine, error) {
										var (
											oldLine, newLine int32  = h.OrigStartLine, h.NewStartLine
											body             []byte = h.Body
										)
										rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
										if len(body) == 0 {
											return []patchLine{}, nil
										}
										lines := make([]patchLine, 0, len(rawLines))
										for _, raw := range rawLines {
											if len(raw) == 0 {
												return nil, errors.New("malformed empty diff line")
											}
											text := func() string {
												var value string = string(raw[1:])
												return strings.ReplaceAll(func() string {
													var result strings.Builder
													for _, r := range value {
														switch {
														case r == '\n' || r == '\t':
															_, _ = result.WriteRune(r)
														case r == '\r':
															_, _ = result.WriteString(`\r`)
														case r < 0x20 || r == 0x7f:
															fmt.Fprintf(&result, `\x%02x`, r)
														default:
															_, _ = result.WriteRune(r)
														}
													}
													return result.String()
												}(), "\n", `\n`)
											}()
											switch raw[0] {
											case ' ':
												lines = append(lines, patchLine{Kind: patchContext, Text: text, OldNumber: patchLineNumber(oldLine), NewNumber: patchLineNumber(newLine)})
												oldLine++
												newLine++
											case '+':
												lines = append(lines, patchLine{Kind: patchAddition, Text: text, NewNumber: patchLineNumber(newLine)})
												newLine++
											case '-':
												lines = append(lines, patchLine{Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(oldLine)})
												oldLine++
											case '\\':
											default:
												return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
											}
										}
										return lines, nil
									}()
									if err != nil {
										return patchFile{}, fmt.Errorf("%s: %w", display, err)
									}
									file.Hunks = append(file.Hunks, patchHunk{Header: func() string {
										header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
										if h.Section != "" {
											header += " " + h.Section
										}
										return header
									}(), Lines: lines})
								}
								return file, nil
							}()
							if err != nil {
								return nil, err
							}
							files = append(files, file)
						}
						return files, nil
					}()
					if err != nil {
						return patchPatch{}, err
					}
					untracked, err := func() ([]patchFile, error) {
						out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
						if err != nil {
							return nil, err
						}
						var files []patchFile
						for _, path := range func() []string {
							var paths []string
							for path := range bytes.SplitSeq(out, []byte{0}) {
								if len(path) > 0 {
									paths = append(paths, string(path))
								}
							}
							return paths
						}() {
							if err := ctx.Err(); err != nil {
								return nil, err
							}
							file, include, err := func() (patchFile, bool, error) {
								full := filepath.Join(root, filepath.FromSlash(path))
								info, err := os.Lstat(full)
								if err != nil {
									return patchFile{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
								}
								if info.Mode()&os.ModeSymlink != 0 {
									target, err := os.Readlink(full)
									if err != nil {
										return patchFile{}, false, fmt.Errorf("read symlink %q: %w", path, err)
									}
									return func() patchFile {
										var content string = strings.ReplaceAll(func() string {
											var result strings.Builder
											for _, r := range target {
												switch {
												case r == '\n' || r == '\t':
													_, _ = result.WriteRune(r)
												case r == '\r':
													_, _ = result.WriteString(`\r`)
												case r < 0x20 || r == 0x7f:
													fmt.Fprintf(&result, `\x%02x`, r)
												default:
													_, _ = result.WriteRune(r)
												}
											}
											return result.String()
										}(), "\n", `\n`)
										sourceLines := func() []string {
											var content string = content
											content = strings.TrimSuffix(content, "\n")
											if content == "" {
												return nil
											}
											return strings.Split(content, "\n")
										}()
										lines := make([]patchLine, 0, len(sourceLines))
										for i, line := range sourceLines {
											lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
										}
										return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
											var result strings.Builder
											for _, r := range path {
												switch {
												case r == '\n' || r == '\t':
													_, _ = result.WriteRune(r)
												case r == '\r':
													_, _ = result.WriteString(`\r`)
												case r < 0x20 || r == 0x7f:
													fmt.Fprintf(&result, `\x%02x`, r)
												default:
													_, _ = result.WriteRune(r)
												}
											}
											return result.String()
										}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
									}(), true, nil
								}
								if !info.Mode().IsRegular() {
									return patchFile{}, false, nil
								}
								if info.Size() > patchMaxFileBytes {
									return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range path {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`), Metadata: []string{"untracked file", "content omitted: file exceeds 2 MiB"}}, true, nil
								}
								content, err := os.ReadFile(full)
								if err != nil {
									return patchFile{}, false, fmt.Errorf("read untracked %q: %w", path, err)
								}
								if bytes.IndexByte(content, 0) >= 0 {
									return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range path {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`), Metadata: []string{"untracked binary file"}}, true, nil
								}
								return func() patchFile {
									var content string = func() string {
										var result strings.Builder
										for _, r := range string(content) {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}()
									sourceLines := func() []string {
										var content string = content
										content = strings.TrimSuffix(content, "\n")
										if content == "" {
											return nil
										}
										return strings.Split(content, "\n")
									}()
									lines := make([]patchLine, 0, len(sourceLines))
									for i, line := range sourceLines {
										lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
									}
									return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range path {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
								}(), true, nil
							}()
							if err != nil {
								return nil, err
							}
							if include {
								files = append(files, file)
							}
						}
						return files, nil
					}()
					if err != nil {
						return patchPatch{}, err
					}
					files = append(files, untracked...)
					slices.SortStableFunc(files, func(a, b patchFile) int { return cmp.Compare(a.DisplayPath, b.DisplayPath) })
					if err := ctx.Err(); err != nil {
						return patchPatch{}, err
					}
					return patchPatch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
				}
			}
		})(context.Background(), test.kind)
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, err != nil, true,
			"kind %v: patch = %#v, error = %v", test.kind, got, err,
		)
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, strings.Contains(err.Error(), test.message),
			true, "kind %v: patch = %#v, error = %v", test.kind, got, err,
		)
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, reflect.DeepEqual(got, patchPatch{}), true,
			"kind %v: patch = %#v, error = %v", test.kind, got, err,
		)
	}
}

func TestPatchGetOutsideRepositoryReturnsZeroPatch(t *testing.T) {
	t.Chdir(t.TempDir())
	got, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
		directory, err := os.Getwd()
		if err != nil {
			return patchPatch{}, fmt.Errorf("resolve working directory: %w", err)
		}
		{
			if kind != patchUnstaged && kind != patchBranch {
				return patchPatch{}, fmt.Errorf("unknown patch kind %d", kind)
			}
			if err := ctx.Err(); err != nil {
				return patchPatch{}, err
			}
			root, err := func() (string, error) {
				rootBytes, err := patchRunner(patchExecRunner{}).Run(ctx, directory, "rev-parse", "--show-toplevel")
				if err != nil {
					return "", err
				}
				root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
				if err != nil {
					return "", fmt.Errorf("resolve repository root: %w", err)
				}
				return root, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			branch, err := func() (string, error) {
				out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
				if err == nil {
					return strings.TrimSpace(string(out)), nil
				}
				for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
					_, err := patchRunner(patchExecRunner{}).Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
					if ctx.Err() != nil {
						return "", ctx.Err()
					}
					if err == nil {
						return candidate, nil
					}
				}
				return "", nil
			}()
			if err != nil {
				return patchPatch{}, fmt.Errorf("discover default branch: %w", err)
			}
			revisions, readOld, err := func() ([]string, patchSourceReader, error) {
				if kind != patchBranch {
					return nil, func(ctx context.Context, run patchRunner, root, path string) string {
						if path == "" || path == "/dev/null" {
							return ""
						}
						out, err := run.Run(ctx, root, "show", ":"+path)
						if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
							return ""
						}
						var result strings.Builder
						for _, r := range string(out) {
							switch {
							case r == '\n' || r == '\t':
								_, _ = result.WriteRune(r)
							case r == '\r':
								_, _ = result.WriteString(`\r`)
							case r < 0x20 || r == 0x7f:
								fmt.Fprintf(&result, `\x%02x`, r)
							default:
								_, _ = result.WriteRune(r)
							}
						}
						return result.String()
					}, nil
				}
				if branch == "" {
					return nil, nil, errors.New("default branch is unavailable")
				}
				baseBytes, err := patchRunner(patchExecRunner{}).Run(ctx, root, "merge-base", branch, "HEAD")
				if err != nil {
					return nil, nil, fmt.Errorf("find branch point with %s: %w", branch, err)
				}
				base := strings.TrimSpace(string(baseBytes))
				readOld := func(ctx context.Context, run patchRunner, root, path string) string {
					if path == "" || path == "/dev/null" {
						return ""
					}
					out, err := run.Run(ctx, root, "show", base+":"+path)
					if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
						return ""
					}
					var result strings.Builder
					for _, r := range string(out) {
						switch {
						case r == '\n' || r == '\t':
							_, _ = result.WriteRune(r)
						case r == '\r':
							_, _ = result.WriteString(`\r`)
						case r < 0x20 || r == 0x7f:
							fmt.Fprintf(&result, `\x%02x`, r)
						default:
							_, _ = result.WriteRune(r)
						}
					}
					return result.String()
				}
				return []string{base}, readOld, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			raw, err := func() ([]byte, error) {
				args := []string{"-c", "core.quotepath=false", "-c", "diff.external=", "--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--unified=3"}
				args = append(args, revisions...)
				args = append(args, "--")
				return patchRunner(patchExecRunner{}).Run(ctx, root, args...)
			}()
			if err != nil {
				return patchPatch{}, err
			}
			{
				files, err := func() ([]patchFile, error) {
					if len(bytes.TrimSpace(raw)) == 0 {
						return nil, nil
					}
					parsed, err := diff.ParseMultiFileDiff(raw)
					if err != nil {
						return nil, fmt.Errorf("parse git diff: %w", err)
					}
					files := make([]patchFile, 0, len(parsed))
					for _, fd := range parsed {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, err := func() (patchFile, error) {
							oldPath := func() string {
								var path string = fd.OrigName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							newPath := func() string {
								var path string = fd.NewName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							display := newPath
							if display == "" || display == "/dev/null" {
								display = oldPath
							}
							file := patchFile{OldPath: oldPath, NewPath: newPath, DisplayPath: func() string {
								var value string = display
								return strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range value {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`)
							}(), Metadata: func() []string {
								var values []string = fd.Extended
								result := make([]string, len(values))
								for index, value := range values {
									result[index] = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range value {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
								}
								return result
							}()}
							file.OldSource = readOld(ctx, patchExecRunner{}, root, oldPath)
							file.NewSource = func() string {
								if newPath == "" || newPath == "/dev/null" {
									return ""
								}
								full := filepath.Join(root, filepath.FromSlash(newPath))
								info, err := os.Lstat(full)
								if err != nil || !info.Mode().IsRegular() || info.Size() > patchMaxFileBytes {
									return ""
								}
								out, err := os.ReadFile(full)
								if err != nil || bytes.IndexByte(out, 0) >= 0 {
									return ""
								}
								var result strings.Builder
								for _, r := range string(out) {
									switch {
									case r == '\n' || r == '\t':
										_, _ = result.WriteRune(r)
									case r == '\r':
										_, _ = result.WriteString(`\r`)
									case r < 0x20 || r == 0x7f:
										fmt.Fprintf(&result, `\x%02x`, r)
									default:
										_, _ = result.WriteRune(r)
									}
								}
								return result.String()
							}()
							for _, h := range fd.Hunks {
								lines, err := func() ([]patchLine, error) {
									var (
										oldLine, newLine int32  = h.OrigStartLine, h.NewStartLine
										body             []byte = h.Body
									)
									rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
									if len(body) == 0 {
										return []patchLine{}, nil
									}
									lines := make([]patchLine, 0, len(rawLines))
									for _, raw := range rawLines {
										if len(raw) == 0 {
											return nil, errors.New("malformed empty diff line")
										}
										text := func() string {
											var value string = string(raw[1:])
											return strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range value {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`)
										}()
										switch raw[0] {
										case ' ':
											lines = append(lines, patchLine{Kind: patchContext, Text: text, OldNumber: patchLineNumber(oldLine), NewNumber: patchLineNumber(newLine)})
											oldLine++
											newLine++
										case '+':
											lines = append(lines, patchLine{Kind: patchAddition, Text: text, NewNumber: patchLineNumber(newLine)})
											newLine++
										case '-':
											lines = append(lines, patchLine{Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(oldLine)})
											oldLine++
										case '\\':
										default:
											return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
										}
									}
									return lines, nil
								}()
								if err != nil {
									return patchFile{}, fmt.Errorf("%s: %w", display, err)
								}
								file.Hunks = append(file.Hunks, patchHunk{Header: func() string {
									header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
									if h.Section != "" {
										header += " " + h.Section
									}
									return header
								}(), Lines: lines})
							}
							return file, nil
						}()
						if err != nil {
							return nil, err
						}
						files = append(files, file)
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				untracked, err := func() ([]patchFile, error) {
					out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
					if err != nil {
						return nil, err
					}
					var files []patchFile
					for _, path := range func() []string {
						var paths []string
						for path := range bytes.SplitSeq(out, []byte{0}) {
							if len(path) > 0 {
								paths = append(paths, string(path))
							}
						}
						return paths
					}() {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, include, err := func() (patchFile, bool, error) {
							full := filepath.Join(root, filepath.FromSlash(path))
							info, err := os.Lstat(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
							}
							if info.Mode()&os.ModeSymlink != 0 {
								target, err := os.Readlink(full)
								if err != nil {
									return patchFile{}, false, fmt.Errorf("read symlink %q: %w", path, err)
								}
								return func() patchFile {
									var content string = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range target {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
									sourceLines := func() []string {
										var content string = content
										content = strings.TrimSuffix(content, "\n")
										if content == "" {
											return nil
										}
										return strings.Split(content, "\n")
									}()
									lines := make([]patchLine, 0, len(sourceLines))
									for i, line := range sourceLines {
										lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
									}
									return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range path {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
								}(), true, nil
							}
							if !info.Mode().IsRegular() {
								return patchFile{}, false, nil
							}
							if info.Size() > patchMaxFileBytes {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked file", "content omitted: file exceeds 2 MiB"}}, true, nil
							}
							content, err := os.ReadFile(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("read untracked %q: %w", path, err)
							}
							if bytes.IndexByte(content, 0) >= 0 {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked binary file"}}, true, nil
							}
							return func() patchFile {
								var content string = func() string {
									var result strings.Builder
									for _, r := range string(content) {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}()
								sourceLines := func() []string {
									var content string = content
									content = strings.TrimSuffix(content, "\n")
									if content == "" {
										return nil
									}
									return strings.Split(content, "\n")
								}()
								lines := make([]patchLine, 0, len(sourceLines))
								for i, line := range sourceLines {
									lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
								}
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
							}(), true, nil
						}()
						if err != nil {
							return nil, err
						}
						if include {
							files = append(files, file)
						}
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				files = append(files, untracked...)
				slices.SortStableFunc(files, func(a, b patchFile) int { return cmp.Compare(a.DisplayPath, b.DisplayPath) })
				if err := ctx.Err(); err != nil {
					return patchPatch{}, err
				}
				return patchPatch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
			}
		}
	})(context.Background(), patchUnstaged)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, err != nil, true, "patch = %#v, error = %v", got,
		err,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, reflect.DeepEqual(got, patchPatch{}), true,
		"patch = %#v, error = %v", got, err,
	)
}

func TestPatchGetHonorsCanceledContext(t *testing.T) {
	t.Chdir(func() string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, err)
		var args []string = []string{"init", "-q"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		{
			var args []string = []string{"config", "user.email", "test@example.com"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		{
			var args []string = []string{"config", "user.name", "Test"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		return repo
	}())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
		directory, err := os.Getwd()
		if err != nil {
			return patchPatch{}, fmt.Errorf("resolve working directory: %w", err)
		}
		{
			if kind != patchUnstaged && kind != patchBranch {
				return patchPatch{}, fmt.Errorf("unknown patch kind %d", kind)
			}
			if err := ctx.Err(); err != nil {
				return patchPatch{}, err
			}
			root, err := func() (string, error) {
				rootBytes, err := patchRunner(patchExecRunner{}).Run(ctx, directory, "rev-parse", "--show-toplevel")
				if err != nil {
					return "", err
				}
				root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
				if err != nil {
					return "", fmt.Errorf("resolve repository root: %w", err)
				}
				return root, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			branch, err := func() (string, error) {
				out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
				if err == nil {
					return strings.TrimSpace(string(out)), nil
				}
				for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
					_, err := patchRunner(patchExecRunner{}).Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
					if ctx.Err() != nil {
						return "", ctx.Err()
					}
					if err == nil {
						return candidate, nil
					}
				}
				return "", nil
			}()
			if err != nil {
				return patchPatch{}, fmt.Errorf("discover default branch: %w", err)
			}
			revisions, readOld, err := func() ([]string, patchSourceReader, error) {
				if kind != patchBranch {
					return nil, func(ctx context.Context, run patchRunner, root, path string) string {
						if path == "" || path == "/dev/null" {
							return ""
						}
						out, err := run.Run(ctx, root, "show", ":"+path)
						if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
							return ""
						}
						var result strings.Builder
						for _, r := range string(out) {
							switch {
							case r == '\n' || r == '\t':
								_, _ = result.WriteRune(r)
							case r == '\r':
								_, _ = result.WriteString(`\r`)
							case r < 0x20 || r == 0x7f:
								fmt.Fprintf(&result, `\x%02x`, r)
							default:
								_, _ = result.WriteRune(r)
							}
						}
						return result.String()
					}, nil
				}
				if branch == "" {
					return nil, nil, errors.New("default branch is unavailable")
				}
				baseBytes, err := patchRunner(patchExecRunner{}).Run(ctx, root, "merge-base", branch, "HEAD")
				if err != nil {
					return nil, nil, fmt.Errorf("find branch point with %s: %w", branch, err)
				}
				base := strings.TrimSpace(string(baseBytes))
				readOld := func(ctx context.Context, run patchRunner, root, path string) string {
					if path == "" || path == "/dev/null" {
						return ""
					}
					out, err := run.Run(ctx, root, "show", base+":"+path)
					if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
						return ""
					}
					var result strings.Builder
					for _, r := range string(out) {
						switch {
						case r == '\n' || r == '\t':
							_, _ = result.WriteRune(r)
						case r == '\r':
							_, _ = result.WriteString(`\r`)
						case r < 0x20 || r == 0x7f:
							fmt.Fprintf(&result, `\x%02x`, r)
						default:
							_, _ = result.WriteRune(r)
						}
					}
					return result.String()
				}
				return []string{base}, readOld, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			raw, err := func() ([]byte, error) {
				args := []string{"-c", "core.quotepath=false", "-c", "diff.external=", "--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--unified=3"}
				args = append(args, revisions...)
				args = append(args, "--")
				return patchRunner(patchExecRunner{}).Run(ctx, root, args...)
			}()
			if err != nil {
				return patchPatch{}, err
			}
			{
				files, err := func() ([]patchFile, error) {
					if len(bytes.TrimSpace(raw)) == 0 {
						return nil, nil
					}
					parsed, err := diff.ParseMultiFileDiff(raw)
					if err != nil {
						return nil, fmt.Errorf("parse git diff: %w", err)
					}
					files := make([]patchFile, 0, len(parsed))
					for _, fd := range parsed {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, err := func() (patchFile, error) {
							oldPath := func() string {
								var path string = fd.OrigName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							newPath := func() string {
								var path string = fd.NewName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							display := newPath
							if display == "" || display == "/dev/null" {
								display = oldPath
							}
							file := patchFile{OldPath: oldPath, NewPath: newPath, DisplayPath: func() string {
								var value string = display
								return strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range value {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`)
							}(), Metadata: func() []string {
								var values []string = fd.Extended
								result := make([]string, len(values))
								for index, value := range values {
									result[index] = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range value {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
								}
								return result
							}()}
							file.OldSource = readOld(ctx, patchExecRunner{}, root, oldPath)
							file.NewSource = func() string {
								if newPath == "" || newPath == "/dev/null" {
									return ""
								}
								full := filepath.Join(root, filepath.FromSlash(newPath))
								info, err := os.Lstat(full)
								if err != nil || !info.Mode().IsRegular() || info.Size() > patchMaxFileBytes {
									return ""
								}
								out, err := os.ReadFile(full)
								if err != nil || bytes.IndexByte(out, 0) >= 0 {
									return ""
								}
								var result strings.Builder
								for _, r := range string(out) {
									switch {
									case r == '\n' || r == '\t':
										_, _ = result.WriteRune(r)
									case r == '\r':
										_, _ = result.WriteString(`\r`)
									case r < 0x20 || r == 0x7f:
										fmt.Fprintf(&result, `\x%02x`, r)
									default:
										_, _ = result.WriteRune(r)
									}
								}
								return result.String()
							}()
							for _, h := range fd.Hunks {
								lines, err := func() ([]patchLine, error) {
									var (
										oldLine, newLine int32  = h.OrigStartLine, h.NewStartLine
										body             []byte = h.Body
									)
									rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
									if len(body) == 0 {
										return []patchLine{}, nil
									}
									lines := make([]patchLine, 0, len(rawLines))
									for _, raw := range rawLines {
										if len(raw) == 0 {
											return nil, errors.New("malformed empty diff line")
										}
										text := func() string {
											var value string = string(raw[1:])
											return strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range value {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`)
										}()
										switch raw[0] {
										case ' ':
											lines = append(lines, patchLine{Kind: patchContext, Text: text, OldNumber: patchLineNumber(oldLine), NewNumber: patchLineNumber(newLine)})
											oldLine++
											newLine++
										case '+':
											lines = append(lines, patchLine{Kind: patchAddition, Text: text, NewNumber: patchLineNumber(newLine)})
											newLine++
										case '-':
											lines = append(lines, patchLine{Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(oldLine)})
											oldLine++
										case '\\':
										default:
											return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
										}
									}
									return lines, nil
								}()
								if err != nil {
									return patchFile{}, fmt.Errorf("%s: %w", display, err)
								}
								file.Hunks = append(file.Hunks, patchHunk{Header: func() string {
									header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
									if h.Section != "" {
										header += " " + h.Section
									}
									return header
								}(), Lines: lines})
							}
							return file, nil
						}()
						if err != nil {
							return nil, err
						}
						files = append(files, file)
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				untracked, err := func() ([]patchFile, error) {
					out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
					if err != nil {
						return nil, err
					}
					var files []patchFile
					for _, path := range func() []string {
						var paths []string
						for path := range bytes.SplitSeq(out, []byte{0}) {
							if len(path) > 0 {
								paths = append(paths, string(path))
							}
						}
						return paths
					}() {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, include, err := func() (patchFile, bool, error) {
							full := filepath.Join(root, filepath.FromSlash(path))
							info, err := os.Lstat(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
							}
							if info.Mode()&os.ModeSymlink != 0 {
								target, err := os.Readlink(full)
								if err != nil {
									return patchFile{}, false, fmt.Errorf("read symlink %q: %w", path, err)
								}
								return func() patchFile {
									var content string = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range target {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
									sourceLines := func() []string {
										var content string = content
										content = strings.TrimSuffix(content, "\n")
										if content == "" {
											return nil
										}
										return strings.Split(content, "\n")
									}()
									lines := make([]patchLine, 0, len(sourceLines))
									for i, line := range sourceLines {
										lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
									}
									return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range path {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
								}(), true, nil
							}
							if !info.Mode().IsRegular() {
								return patchFile{}, false, nil
							}
							if info.Size() > patchMaxFileBytes {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked file", "content omitted: file exceeds 2 MiB"}}, true, nil
							}
							content, err := os.ReadFile(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("read untracked %q: %w", path, err)
							}
							if bytes.IndexByte(content, 0) >= 0 {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked binary file"}}, true, nil
							}
							return func() patchFile {
								var content string = func() string {
									var result strings.Builder
									for _, r := range string(content) {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}()
								sourceLines := func() []string {
									var content string = content
									content = strings.TrimSuffix(content, "\n")
									if content == "" {
										return nil
									}
									return strings.Split(content, "\n")
								}()
								lines := make([]patchLine, 0, len(sourceLines))
								for i, line := range sourceLines {
									lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
								}
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
							}(), true, nil
						}()
						if err != nil {
							return nil, err
						}
						if include {
							files = append(files, file)
						}
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				files = append(files, untracked...)
				slices.SortStableFunc(files, func(a, b patchFile) int { return cmp.Compare(a.DisplayPath, b.DisplayPath) })
				if err := ctx.Err(); err != nil {
					return patchPatch{}, err
				}
				return patchPatch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
			}
		}
	})(ctx, patchUnstaged)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, errors.Is(err, context.Canceled), true,
		"patch = %#v, error = %v", got, err,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, reflect.DeepEqual(got, patchPatch{}), true,
		"patch = %#v, error = %v", got, err,
	)
}

func TestPatchGetOmitsLargeUntrackedContents(t *testing.T) {
	repo := func() string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, err)
		var args []string = []string{"init", "-q"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		{
			var args []string = []string{"config", "user.email", "test@example.com"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		{
			var args []string = []string{"config", "user.name", "Test"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		return repo
	}()
	var content string = strings.Repeat("x", patchMaxFileBytes+1)
	t.Helper()
	path := filepath.Join(repo, "large.txt")
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, os.MkdirAll(filepath.Dir(path), 0o755))
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, os.WriteFile(path, []byte(content), 0o644))
	t.Chdir(repo)
	got, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
		directory, err := os.Getwd()
		if err != nil {
			return patchPatch{}, fmt.Errorf("resolve working directory: %w", err)
		}
		{
			if kind != patchUnstaged && kind != patchBranch {
				return patchPatch{}, fmt.Errorf("unknown patch kind %d", kind)
			}
			if err := ctx.Err(); err != nil {
				return patchPatch{}, err
			}
			root, err := func() (string, error) {
				rootBytes, err := patchRunner(patchExecRunner{}).Run(ctx, directory, "rev-parse", "--show-toplevel")
				if err != nil {
					return "", err
				}
				root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
				if err != nil {
					return "", fmt.Errorf("resolve repository root: %w", err)
				}
				return root, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			branch, err := func() (string, error) {
				out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
				if err == nil {
					return strings.TrimSpace(string(out)), nil
				}
				for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
					_, err := patchRunner(patchExecRunner{}).Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
					if ctx.Err() != nil {
						return "", ctx.Err()
					}
					if err == nil {
						return candidate, nil
					}
				}
				return "", nil
			}()
			if err != nil {
				return patchPatch{}, fmt.Errorf("discover default branch: %w", err)
			}
			revisions, readOld, err := func() ([]string, patchSourceReader, error) {
				if kind != patchBranch {
					return nil, func(ctx context.Context, run patchRunner, root, path string) string {
						if path == "" || path == "/dev/null" {
							return ""
						}
						out, err := run.Run(ctx, root, "show", ":"+path)
						if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
							return ""
						}
						var result strings.Builder
						for _, r := range string(out) {
							switch {
							case r == '\n' || r == '\t':
								_, _ = result.WriteRune(r)
							case r == '\r':
								_, _ = result.WriteString(`\r`)
							case r < 0x20 || r == 0x7f:
								fmt.Fprintf(&result, `\x%02x`, r)
							default:
								_, _ = result.WriteRune(r)
							}
						}
						return result.String()
					}, nil
				}
				if branch == "" {
					return nil, nil, errors.New("default branch is unavailable")
				}
				baseBytes, err := patchRunner(patchExecRunner{}).Run(ctx, root, "merge-base", branch, "HEAD")
				if err != nil {
					return nil, nil, fmt.Errorf("find branch point with %s: %w", branch, err)
				}
				base := strings.TrimSpace(string(baseBytes))
				readOld := func(ctx context.Context, run patchRunner, root, path string) string {
					if path == "" || path == "/dev/null" {
						return ""
					}
					out, err := run.Run(ctx, root, "show", base+":"+path)
					if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
						return ""
					}
					var result strings.Builder
					for _, r := range string(out) {
						switch {
						case r == '\n' || r == '\t':
							_, _ = result.WriteRune(r)
						case r == '\r':
							_, _ = result.WriteString(`\r`)
						case r < 0x20 || r == 0x7f:
							fmt.Fprintf(&result, `\x%02x`, r)
						default:
							_, _ = result.WriteRune(r)
						}
					}
					return result.String()
				}
				return []string{base}, readOld, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			raw, err := func() ([]byte, error) {
				args := []string{"-c", "core.quotepath=false", "-c", "diff.external=", "--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--unified=3"}
				args = append(args, revisions...)
				args = append(args, "--")
				return patchRunner(patchExecRunner{}).Run(ctx, root, args...)
			}()
			if err != nil {
				return patchPatch{}, err
			}
			{
				files, err := func() ([]patchFile, error) {
					if len(bytes.TrimSpace(raw)) == 0 {
						return nil, nil
					}
					parsed, err := diff.ParseMultiFileDiff(raw)
					if err != nil {
						return nil, fmt.Errorf("parse git diff: %w", err)
					}
					files := make([]patchFile, 0, len(parsed))
					for _, fd := range parsed {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, err := func() (patchFile, error) {
							oldPath := func() string {
								var path string = fd.OrigName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							newPath := func() string {
								var path string = fd.NewName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							display := newPath
							if display == "" || display == "/dev/null" {
								display = oldPath
							}
							file := patchFile{OldPath: oldPath, NewPath: newPath, DisplayPath: func() string {
								var value string = display
								return strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range value {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`)
							}(), Metadata: func() []string {
								var values []string = fd.Extended
								result := make([]string, len(values))
								for index, value := range values {
									result[index] = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range value {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
								}
								return result
							}()}
							file.OldSource = readOld(ctx, patchExecRunner{}, root, oldPath)
							file.NewSource = func() string {
								if newPath == "" || newPath == "/dev/null" {
									return ""
								}
								full := filepath.Join(root, filepath.FromSlash(newPath))
								info, err := os.Lstat(full)
								if err != nil || !info.Mode().IsRegular() || info.Size() > patchMaxFileBytes {
									return ""
								}
								out, err := os.ReadFile(full)
								if err != nil || bytes.IndexByte(out, 0) >= 0 {
									return ""
								}
								var result strings.Builder
								for _, r := range string(out) {
									switch {
									case r == '\n' || r == '\t':
										_, _ = result.WriteRune(r)
									case r == '\r':
										_, _ = result.WriteString(`\r`)
									case r < 0x20 || r == 0x7f:
										fmt.Fprintf(&result, `\x%02x`, r)
									default:
										_, _ = result.WriteRune(r)
									}
								}
								return result.String()
							}()
							for _, h := range fd.Hunks {
								lines, err := func() ([]patchLine, error) {
									var (
										oldLine, newLine int32  = h.OrigStartLine, h.NewStartLine
										body             []byte = h.Body
									)
									rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
									if len(body) == 0 {
										return []patchLine{}, nil
									}
									lines := make([]patchLine, 0, len(rawLines))
									for _, raw := range rawLines {
										if len(raw) == 0 {
											return nil, errors.New("malformed empty diff line")
										}
										text := func() string {
											var value string = string(raw[1:])
											return strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range value {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`)
										}()
										switch raw[0] {
										case ' ':
											lines = append(lines, patchLine{Kind: patchContext, Text: text, OldNumber: patchLineNumber(oldLine), NewNumber: patchLineNumber(newLine)})
											oldLine++
											newLine++
										case '+':
											lines = append(lines, patchLine{Kind: patchAddition, Text: text, NewNumber: patchLineNumber(newLine)})
											newLine++
										case '-':
											lines = append(lines, patchLine{Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(oldLine)})
											oldLine++
										case '\\':
										default:
											return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
										}
									}
									return lines, nil
								}()
								if err != nil {
									return patchFile{}, fmt.Errorf("%s: %w", display, err)
								}
								file.Hunks = append(file.Hunks, patchHunk{Header: func() string {
									header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
									if h.Section != "" {
										header += " " + h.Section
									}
									return header
								}(), Lines: lines})
							}
							return file, nil
						}()
						if err != nil {
							return nil, err
						}
						files = append(files, file)
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				untracked, err := func() ([]patchFile, error) {
					out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
					if err != nil {
						return nil, err
					}
					var files []patchFile
					for _, path := range func() []string {
						var paths []string
						for path := range bytes.SplitSeq(out, []byte{0}) {
							if len(path) > 0 {
								paths = append(paths, string(path))
							}
						}
						return paths
					}() {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, include, err := func() (patchFile, bool, error) {
							full := filepath.Join(root, filepath.FromSlash(path))
							info, err := os.Lstat(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
							}
							if info.Mode()&os.ModeSymlink != 0 {
								target, err := os.Readlink(full)
								if err != nil {
									return patchFile{}, false, fmt.Errorf("read symlink %q: %w", path, err)
								}
								return func() patchFile {
									var content string = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range target {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
									sourceLines := func() []string {
										var content string = content
										content = strings.TrimSuffix(content, "\n")
										if content == "" {
											return nil
										}
										return strings.Split(content, "\n")
									}()
									lines := make([]patchLine, 0, len(sourceLines))
									for i, line := range sourceLines {
										lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
									}
									return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range path {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
								}(), true, nil
							}
							if !info.Mode().IsRegular() {
								return patchFile{}, false, nil
							}
							if info.Size() > patchMaxFileBytes {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked file", "content omitted: file exceeds 2 MiB"}}, true, nil
							}
							content, err := os.ReadFile(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("read untracked %q: %w", path, err)
							}
							if bytes.IndexByte(content, 0) >= 0 {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked binary file"}}, true, nil
							}
							return func() patchFile {
								var content string = func() string {
									var result strings.Builder
									for _, r := range string(content) {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}()
								sourceLines := func() []string {
									var content string = content
									content = strings.TrimSuffix(content, "\n")
									if content == "" {
										return nil
									}
									return strings.Split(content, "\n")
								}()
								lines := make([]patchLine, 0, len(sourceLines))
								for i, line := range sourceLines {
									lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
								}
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
							}(), true, nil
						}()
						if err != nil {
							return nil, err
						}
						if include {
							files = append(files, file)
						}
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				files = append(files, untracked...)
				slices.SortStableFunc(files, func(a, b patchFile) int { return cmp.Compare(a.DisplayPath, b.DisplayPath) })
				if err := ctx.Err(); err != nil {
					return patchPatch{}, err
				}
				return patchPatch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
			}
		}
	})(context.Background(), patchUnstaged)
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(got.Files), 1, "files = %#v", got.Files)

	file := got.Files[0]
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, file.OldPath, "", "large file = %#v", file)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, file.NewPath, "large.txt", "large file = %#v",
		file,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, file.NewSource, "", "large file = %#v", file)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(file.Hunks), 0, "large file = %#v", file)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		strings.Contains(
			strings.Join(file.Metadata, "\n"),
			"exceeds 2 MiB",
		), true, "large file = %#v", file)
}

func TestPatchGetShowsBinaryMetadataWithoutBinaryDiffLines(t *testing.T) {
	repo := func() string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, err)
		var args []string = []string{"init", "-q"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		{
			var args []string = []string{"config", "user.email", "test@example.com"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		{
			var args []string = []string{"config", "user.name", "Test"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		return repo
	}()
	t.Helper()
	path := filepath.Join(repo, "tracked.bin")
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, os.MkdirAll(filepath.Dir(path), 0o755))
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, os.WriteFile(path, []byte("\x00old"), 0o644))
	var args []string = []string{"add", "tracked.bin"}
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	{
		var args []string = []string{"commit", "-m", "base"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		t.Helper()
		path := filepath.Join(repo, "tracked.bin")
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.MkdirAll(filepath.Dir(path), 0o755))
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.WriteFile(path, []byte("\x00new"), 0o644))
	}
	{
		t.Helper()
		path := filepath.Join(repo, "untracked.bin")
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.MkdirAll(filepath.Dir(path), 0o755))
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.WriteFile(path, []byte("\x00content"), 0o644))
	}

	t.Chdir(repo)
	got, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
		directory, err := os.Getwd()
		if err != nil {
			return patchPatch{}, fmt.Errorf("resolve working directory: %w", err)
		}
		{
			if kind != patchUnstaged && kind != patchBranch {
				return patchPatch{}, fmt.Errorf("unknown patch kind %d", kind)
			}
			if err := ctx.Err(); err != nil {
				return patchPatch{}, err
			}
			root, err := func() (string, error) {
				rootBytes, err := patchRunner(patchExecRunner{}).Run(ctx, directory, "rev-parse", "--show-toplevel")
				if err != nil {
					return "", err
				}
				root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
				if err != nil {
					return "", fmt.Errorf("resolve repository root: %w", err)
				}
				return root, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			branch, err := func() (string, error) {
				out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
				if err == nil {
					return strings.TrimSpace(string(out)), nil
				}
				for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
					_, err := patchRunner(patchExecRunner{}).Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
					if ctx.Err() != nil {
						return "", ctx.Err()
					}
					if err == nil {
						return candidate, nil
					}
				}
				return "", nil
			}()
			if err != nil {
				return patchPatch{}, fmt.Errorf("discover default branch: %w", err)
			}
			revisions, readOld, err := func() ([]string, patchSourceReader, error) {
				if kind != patchBranch {
					return nil, func(ctx context.Context, run patchRunner, root, path string) string {
						if path == "" || path == "/dev/null" {
							return ""
						}
						out, err := run.Run(ctx, root, "show", ":"+path)
						if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
							return ""
						}
						var result strings.Builder
						for _, r := range string(out) {
							switch {
							case r == '\n' || r == '\t':
								_, _ = result.WriteRune(r)
							case r == '\r':
								_, _ = result.WriteString(`\r`)
							case r < 0x20 || r == 0x7f:
								fmt.Fprintf(&result, `\x%02x`, r)
							default:
								_, _ = result.WriteRune(r)
							}
						}
						return result.String()
					}, nil
				}
				if branch == "" {
					return nil, nil, errors.New("default branch is unavailable")
				}
				baseBytes, err := patchRunner(patchExecRunner{}).Run(ctx, root, "merge-base", branch, "HEAD")
				if err != nil {
					return nil, nil, fmt.Errorf("find branch point with %s: %w", branch, err)
				}
				base := strings.TrimSpace(string(baseBytes))
				readOld := func(ctx context.Context, run patchRunner, root, path string) string {
					if path == "" || path == "/dev/null" {
						return ""
					}
					out, err := run.Run(ctx, root, "show", base+":"+path)
					if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
						return ""
					}
					var result strings.Builder
					for _, r := range string(out) {
						switch {
						case r == '\n' || r == '\t':
							_, _ = result.WriteRune(r)
						case r == '\r':
							_, _ = result.WriteString(`\r`)
						case r < 0x20 || r == 0x7f:
							fmt.Fprintf(&result, `\x%02x`, r)
						default:
							_, _ = result.WriteRune(r)
						}
					}
					return result.String()
				}
				return []string{base}, readOld, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			raw, err := func() ([]byte, error) {
				args := []string{"-c", "core.quotepath=false", "-c", "diff.external=", "--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--unified=3"}
				args = append(args, revisions...)
				args = append(args, "--")
				return patchRunner(patchExecRunner{}).Run(ctx, root, args...)
			}()
			if err != nil {
				return patchPatch{}, err
			}
			{
				files, err := func() ([]patchFile, error) {
					if len(bytes.TrimSpace(raw)) == 0 {
						return nil, nil
					}
					parsed, err := diff.ParseMultiFileDiff(raw)
					if err != nil {
						return nil, fmt.Errorf("parse git diff: %w", err)
					}
					files := make([]patchFile, 0, len(parsed))
					for _, fd := range parsed {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, err := func() (patchFile, error) {
							oldPath := func() string {
								var path string = fd.OrigName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							newPath := func() string {
								var path string = fd.NewName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							display := newPath
							if display == "" || display == "/dev/null" {
								display = oldPath
							}
							file := patchFile{OldPath: oldPath, NewPath: newPath, DisplayPath: func() string {
								var value string = display
								return strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range value {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`)
							}(), Metadata: func() []string {
								var values []string = fd.Extended
								result := make([]string, len(values))
								for index, value := range values {
									result[index] = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range value {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
								}
								return result
							}()}
							file.OldSource = readOld(ctx, patchExecRunner{}, root, oldPath)
							file.NewSource = func() string {
								if newPath == "" || newPath == "/dev/null" {
									return ""
								}
								full := filepath.Join(root, filepath.FromSlash(newPath))
								info, err := os.Lstat(full)
								if err != nil || !info.Mode().IsRegular() || info.Size() > patchMaxFileBytes {
									return ""
								}
								out, err := os.ReadFile(full)
								if err != nil || bytes.IndexByte(out, 0) >= 0 {
									return ""
								}
								var result strings.Builder
								for _, r := range string(out) {
									switch {
									case r == '\n' || r == '\t':
										_, _ = result.WriteRune(r)
									case r == '\r':
										_, _ = result.WriteString(`\r`)
									case r < 0x20 || r == 0x7f:
										fmt.Fprintf(&result, `\x%02x`, r)
									default:
										_, _ = result.WriteRune(r)
									}
								}
								return result.String()
							}()
							for _, h := range fd.Hunks {
								lines, err := func() ([]patchLine, error) {
									var (
										oldLine, newLine int32  = h.OrigStartLine, h.NewStartLine
										body             []byte = h.Body
									)
									rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
									if len(body) == 0 {
										return []patchLine{}, nil
									}
									lines := make([]patchLine, 0, len(rawLines))
									for _, raw := range rawLines {
										if len(raw) == 0 {
											return nil, errors.New("malformed empty diff line")
										}
										text := func() string {
											var value string = string(raw[1:])
											return strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range value {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`)
										}()
										switch raw[0] {
										case ' ':
											lines = append(lines, patchLine{Kind: patchContext, Text: text, OldNumber: patchLineNumber(oldLine), NewNumber: patchLineNumber(newLine)})
											oldLine++
											newLine++
										case '+':
											lines = append(lines, patchLine{Kind: patchAddition, Text: text, NewNumber: patchLineNumber(newLine)})
											newLine++
										case '-':
											lines = append(lines, patchLine{Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(oldLine)})
											oldLine++
										case '\\':
										default:
											return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
										}
									}
									return lines, nil
								}()
								if err != nil {
									return patchFile{}, fmt.Errorf("%s: %w", display, err)
								}
								file.Hunks = append(file.Hunks, patchHunk{Header: func() string {
									header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
									if h.Section != "" {
										header += " " + h.Section
									}
									return header
								}(), Lines: lines})
							}
							return file, nil
						}()
						if err != nil {
							return nil, err
						}
						files = append(files, file)
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				untracked, err := func() ([]patchFile, error) {
					out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
					if err != nil {
						return nil, err
					}
					var files []patchFile
					for _, path := range func() []string {
						var paths []string
						for path := range bytes.SplitSeq(out, []byte{0}) {
							if len(path) > 0 {
								paths = append(paths, string(path))
							}
						}
						return paths
					}() {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, include, err := func() (patchFile, bool, error) {
							full := filepath.Join(root, filepath.FromSlash(path))
							info, err := os.Lstat(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
							}
							if info.Mode()&os.ModeSymlink != 0 {
								target, err := os.Readlink(full)
								if err != nil {
									return patchFile{}, false, fmt.Errorf("read symlink %q: %w", path, err)
								}
								return func() patchFile {
									var content string = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range target {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
									sourceLines := func() []string {
										var content string = content
										content = strings.TrimSuffix(content, "\n")
										if content == "" {
											return nil
										}
										return strings.Split(content, "\n")
									}()
									lines := make([]patchLine, 0, len(sourceLines))
									for i, line := range sourceLines {
										lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
									}
									return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range path {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
								}(), true, nil
							}
							if !info.Mode().IsRegular() {
								return patchFile{}, false, nil
							}
							if info.Size() > patchMaxFileBytes {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked file", "content omitted: file exceeds 2 MiB"}}, true, nil
							}
							content, err := os.ReadFile(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("read untracked %q: %w", path, err)
							}
							if bytes.IndexByte(content, 0) >= 0 {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked binary file"}}, true, nil
							}
							return func() patchFile {
								var content string = func() string {
									var result strings.Builder
									for _, r := range string(content) {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}()
								sourceLines := func() []string {
									var content string = content
									content = strings.TrimSuffix(content, "\n")
									if content == "" {
										return nil
									}
									return strings.Split(content, "\n")
								}()
								lines := make([]patchLine, 0, len(sourceLines))
								for i, line := range sourceLines {
									lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
								}
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
							}(), true, nil
						}()
						if err != nil {
							return nil, err
						}
						if include {
							files = append(files, file)
						}
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				files = append(files, untracked...)
				slices.SortStableFunc(files, func(a, b patchFile) int { return cmp.Compare(a.DisplayPath, b.DisplayPath) })
				if err := ctx.Err(); err != nil {
					return patchPatch{}, err
				}
				return patchPatch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
			}
		}
	})(context.Background(), patchUnstaged)
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(got.Files), 2, "files = %d, want 2: %#v",
		len(got.Files), got.Files,
	)

	for _, file := range got.Files {
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Errorf(format, args...)
			}
		})(
			t,
			file.NewPath == file.DisplayPath &&
				(file.DisplayPath != "tracked.bin" ||
					file.OldPath == "tracked.bin"),
			true,
			"binary paths = %#v",
			file,
		)
		(func(t *testing.T, got, want int, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, len(file.Hunks), 0,
			"binary file %q has diff lines: %#v", file.DisplayPath, file.Hunks,
		)
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(
			t,
			strings.Contains(
				strings.ToLower(strings.Join(file.Metadata, "\n")),
				"binary",
			),
			true,
			"binary file %q lacks metadata: %#v",
			file.DisplayPath,
			file.Metadata,
		)
	}
}

func TestPatchGetBranchIncludesCommittedStagedUnstagedAndUntrackedChanges(
	t *testing.T,
) {
	repo := func() string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, err)
		var args []string = []string{"init", "-q"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		{
			var args []string = []string{"config", "user.email", "test@example.com"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		{
			var args []string = []string{"config", "user.name", "Test"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		return repo
	}()
	var args []string = []string{"branch", "-M", "main"}
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	t.Helper()
	path := filepath.Join(repo, "committed.txt")
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, os.MkdirAll(filepath.Dir(path), 0o755))
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, os.WriteFile(path, []byte("base\n"), 0o644))
	{
		t.Helper()
		path := filepath.Join(repo, "mixed.txt")
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.MkdirAll(filepath.Dir(path), 0o755))
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.WriteFile(path, []byte("base\n"), 0o644))
	}
	{
		t.Helper()
		path := filepath.Join(repo, "staged.txt")
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.MkdirAll(filepath.Dir(path), 0o755))
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.WriteFile(path, []byte("base\n"), 0o644))
	}
	{
		var args []string = []string{"add", "."}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		var args []string = []string{"commit", "-m", "base"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	{
		var args []string = []string{"switch", "-c", "feature"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		t.Helper()
		path := filepath.Join(repo, "committed.txt")
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.MkdirAll(filepath.Dir(path), 0o755))
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.WriteFile(path, []byte("committed on feature\n"), 0o644))
	}
	{
		var args []string = []string{"add", "committed.txt"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		var args []string = []string{"commit", "-m", "feature commit"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		t.Helper()
		path := filepath.Join(repo, "staged.txt")
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.MkdirAll(filepath.Dir(path), 0o755))
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.WriteFile(path, []byte("staged on feature\n"), 0o644))
	}
	{
		var args []string = []string{"add", "staged.txt"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		t.Helper()
		path := filepath.Join(repo, "mixed.txt")
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.MkdirAll(filepath.Dir(path), 0o755))
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.WriteFile(path, []byte("unstaged on feature\n"), 0o644))
	}
	{
		t.Helper()
		path := filepath.Join(repo, "untracked.txt")
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.MkdirAll(filepath.Dir(path), 0o755))
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.WriteFile(path, []byte("untracked on feature\n"), 0o644))
	}

	t.Chdir(repo)
	got, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
		directory, err := os.Getwd()
		if err != nil {
			return patchPatch{}, fmt.Errorf("resolve working directory: %w", err)
		}
		{
			if kind != patchUnstaged && kind != patchBranch {
				return patchPatch{}, fmt.Errorf("unknown patch kind %d", kind)
			}
			if err := ctx.Err(); err != nil {
				return patchPatch{}, err
			}
			root, err := func() (string, error) {
				rootBytes, err := patchRunner(patchExecRunner{}).Run(ctx, directory, "rev-parse", "--show-toplevel")
				if err != nil {
					return "", err
				}
				root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
				if err != nil {
					return "", fmt.Errorf("resolve repository root: %w", err)
				}
				return root, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			branch, err := func() (string, error) {
				out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
				if err == nil {
					return strings.TrimSpace(string(out)), nil
				}
				for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
					_, err := patchRunner(patchExecRunner{}).Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
					if ctx.Err() != nil {
						return "", ctx.Err()
					}
					if err == nil {
						return candidate, nil
					}
				}
				return "", nil
			}()
			if err != nil {
				return patchPatch{}, fmt.Errorf("discover default branch: %w", err)
			}
			revisions, readOld, err := func() ([]string, patchSourceReader, error) {
				if kind != patchBranch {
					return nil, func(ctx context.Context, run patchRunner, root, path string) string {
						if path == "" || path == "/dev/null" {
							return ""
						}
						out, err := run.Run(ctx, root, "show", ":"+path)
						if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
							return ""
						}
						var result strings.Builder
						for _, r := range string(out) {
							switch {
							case r == '\n' || r == '\t':
								_, _ = result.WriteRune(r)
							case r == '\r':
								_, _ = result.WriteString(`\r`)
							case r < 0x20 || r == 0x7f:
								fmt.Fprintf(&result, `\x%02x`, r)
							default:
								_, _ = result.WriteRune(r)
							}
						}
						return result.String()
					}, nil
				}
				if branch == "" {
					return nil, nil, errors.New("default branch is unavailable")
				}
				baseBytes, err := patchRunner(patchExecRunner{}).Run(ctx, root, "merge-base", branch, "HEAD")
				if err != nil {
					return nil, nil, fmt.Errorf("find branch point with %s: %w", branch, err)
				}
				base := strings.TrimSpace(string(baseBytes))
				readOld := func(ctx context.Context, run patchRunner, root, path string) string {
					if path == "" || path == "/dev/null" {
						return ""
					}
					out, err := run.Run(ctx, root, "show", base+":"+path)
					if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
						return ""
					}
					var result strings.Builder
					for _, r := range string(out) {
						switch {
						case r == '\n' || r == '\t':
							_, _ = result.WriteRune(r)
						case r == '\r':
							_, _ = result.WriteString(`\r`)
						case r < 0x20 || r == 0x7f:
							fmt.Fprintf(&result, `\x%02x`, r)
						default:
							_, _ = result.WriteRune(r)
						}
					}
					return result.String()
				}
				return []string{base}, readOld, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			raw, err := func() ([]byte, error) {
				args := []string{"-c", "core.quotepath=false", "-c", "diff.external=", "--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--unified=3"}
				args = append(args, revisions...)
				args = append(args, "--")
				return patchRunner(patchExecRunner{}).Run(ctx, root, args...)
			}()
			if err != nil {
				return patchPatch{}, err
			}
			{
				files, err := func() ([]patchFile, error) {
					if len(bytes.TrimSpace(raw)) == 0 {
						return nil, nil
					}
					parsed, err := diff.ParseMultiFileDiff(raw)
					if err != nil {
						return nil, fmt.Errorf("parse git diff: %w", err)
					}
					files := make([]patchFile, 0, len(parsed))
					for _, fd := range parsed {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, err := func() (patchFile, error) {
							oldPath := func() string {
								var path string = fd.OrigName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							newPath := func() string {
								var path string = fd.NewName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							display := newPath
							if display == "" || display == "/dev/null" {
								display = oldPath
							}
							file := patchFile{OldPath: oldPath, NewPath: newPath, DisplayPath: func() string {
								var value string = display
								return strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range value {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`)
							}(), Metadata: func() []string {
								var values []string = fd.Extended
								result := make([]string, len(values))
								for index, value := range values {
									result[index] = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range value {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
								}
								return result
							}()}
							file.OldSource = readOld(ctx, patchExecRunner{}, root, oldPath)
							file.NewSource = func() string {
								if newPath == "" || newPath == "/dev/null" {
									return ""
								}
								full := filepath.Join(root, filepath.FromSlash(newPath))
								info, err := os.Lstat(full)
								if err != nil || !info.Mode().IsRegular() || info.Size() > patchMaxFileBytes {
									return ""
								}
								out, err := os.ReadFile(full)
								if err != nil || bytes.IndexByte(out, 0) >= 0 {
									return ""
								}
								var result strings.Builder
								for _, r := range string(out) {
									switch {
									case r == '\n' || r == '\t':
										_, _ = result.WriteRune(r)
									case r == '\r':
										_, _ = result.WriteString(`\r`)
									case r < 0x20 || r == 0x7f:
										fmt.Fprintf(&result, `\x%02x`, r)
									default:
										_, _ = result.WriteRune(r)
									}
								}
								return result.String()
							}()
							for _, h := range fd.Hunks {
								lines, err := func() ([]patchLine, error) {
									var (
										oldLine, newLine int32  = h.OrigStartLine, h.NewStartLine
										body             []byte = h.Body
									)
									rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
									if len(body) == 0 {
										return []patchLine{}, nil
									}
									lines := make([]patchLine, 0, len(rawLines))
									for _, raw := range rawLines {
										if len(raw) == 0 {
											return nil, errors.New("malformed empty diff line")
										}
										text := func() string {
											var value string = string(raw[1:])
											return strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range value {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`)
										}()
										switch raw[0] {
										case ' ':
											lines = append(lines, patchLine{Kind: patchContext, Text: text, OldNumber: patchLineNumber(oldLine), NewNumber: patchLineNumber(newLine)})
											oldLine++
											newLine++
										case '+':
											lines = append(lines, patchLine{Kind: patchAddition, Text: text, NewNumber: patchLineNumber(newLine)})
											newLine++
										case '-':
											lines = append(lines, patchLine{Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(oldLine)})
											oldLine++
										case '\\':
										default:
											return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
										}
									}
									return lines, nil
								}()
								if err != nil {
									return patchFile{}, fmt.Errorf("%s: %w", display, err)
								}
								file.Hunks = append(file.Hunks, patchHunk{Header: func() string {
									header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
									if h.Section != "" {
										header += " " + h.Section
									}
									return header
								}(), Lines: lines})
							}
							return file, nil
						}()
						if err != nil {
							return nil, err
						}
						files = append(files, file)
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				untracked, err := func() ([]patchFile, error) {
					out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
					if err != nil {
						return nil, err
					}
					var files []patchFile
					for _, path := range func() []string {
						var paths []string
						for path := range bytes.SplitSeq(out, []byte{0}) {
							if len(path) > 0 {
								paths = append(paths, string(path))
							}
						}
						return paths
					}() {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, include, err := func() (patchFile, bool, error) {
							full := filepath.Join(root, filepath.FromSlash(path))
							info, err := os.Lstat(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
							}
							if info.Mode()&os.ModeSymlink != 0 {
								target, err := os.Readlink(full)
								if err != nil {
									return patchFile{}, false, fmt.Errorf("read symlink %q: %w", path, err)
								}
								return func() patchFile {
									var content string = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range target {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
									sourceLines := func() []string {
										var content string = content
										content = strings.TrimSuffix(content, "\n")
										if content == "" {
											return nil
										}
										return strings.Split(content, "\n")
									}()
									lines := make([]patchLine, 0, len(sourceLines))
									for i, line := range sourceLines {
										lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
									}
									return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range path {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
								}(), true, nil
							}
							if !info.Mode().IsRegular() {
								return patchFile{}, false, nil
							}
							if info.Size() > patchMaxFileBytes {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked file", "content omitted: file exceeds 2 MiB"}}, true, nil
							}
							content, err := os.ReadFile(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("read untracked %q: %w", path, err)
							}
							if bytes.IndexByte(content, 0) >= 0 {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked binary file"}}, true, nil
							}
							return func() patchFile {
								var content string = func() string {
									var result strings.Builder
									for _, r := range string(content) {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}()
								sourceLines := func() []string {
									var content string = content
									content = strings.TrimSuffix(content, "\n")
									if content == "" {
										return nil
									}
									return strings.Split(content, "\n")
								}()
								lines := make([]patchLine, 0, len(sourceLines))
								for i, line := range sourceLines {
									lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
								}
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
							}(), true, nil
						}()
						if err != nil {
							return nil, err
						}
						if include {
							files = append(files, file)
						}
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				files = append(files, untracked...)
				slices.SortStableFunc(files, func(a, b patchFile) int { return cmp.Compare(a.DisplayPath, b.DisplayPath) })
				if err := ctx.Err(); err != nil {
					return patchPatch{}, err
				}
				return patchPatch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
			}
		}
	})(context.Background(), patchBranch)
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got.Root, repo, "patch metadata = %#v", got)
	(func(t *testing.T, got, want patchKind, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got.Kind, patchBranch, "patch metadata = %#v", got)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got.Branch, "main", "patch metadata = %#v", got)

	want := []string{
		"committed.txt",
		"mixed.txt",
		"staged.txt",
		"untracked.txt",
	}
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(got.Files), len(want),
		"files = %d, want %d: %#v", len(got.Files), len(want), got.Files,
	)

	for i, name := range want {
		(func(t *testing.T, got, want string, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, got.Files[i].DisplayPath, name,
			"file %d = %q, want %q", i, got.Files[i].DisplayPath, name,
		)
	}
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		func() bool {
			for _, hunk := range got.Files[0].Hunks {
				for _, line := range hunk.Lines {
					if line.Kind == patchAddition && strings.Contains(line.Text, "committed on feature") {
						return true
					}
				}
			}
			return false
		}(), true,
		"branch diff lacks expected changes: %#v", got.Files,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		func() bool {
			for _, hunk := range got.Files[1].Hunks {
				for _, line := range hunk.Lines {
					if line.Kind == patchAddition && strings.Contains(line.Text, "unstaged on feature") {
						return true
					}
				}
			}
			return false
		}(), true, "branch diff lacks expected changes: %#v", got.Files)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,

		func() bool {
			for _, hunk := range got.Files[2].Hunks {
				for _, line := range hunk.Lines {
					if line.Kind == patchAddition && strings.Contains(line.Text, "staged on feature") {
						return true
					}
				}
			}
			return false
		}(), true, "branch diff lacks expected changes: %#v", got.Files)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,

		func() bool {
			for _, hunk := range got.Files[3].Hunks {
				for _, line := range hunk.Lines {
					if line.Kind == patchAddition && strings.Contains(line.Text, "untracked on feature") {
						return true
					}
				}
			}
			return false
		}(), true, "branch diff lacks expected changes: %#v", got.Files)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got.Files[0].OldSource, "base\n",
		"unexpected branch sources: old=%q new=%q", got.Files[0].OldSource,
		got.Files[0].NewSource,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got.Files[0].NewSource, "committed on feature\n",
		"unexpected branch sources: old=%q new=%q", got.Files[0].OldSource,
		got.Files[0].NewSource,
	)
}

func TestPatchDefaultBranchFallbacks(t *testing.T) {
	tests := []struct {
		name       string
		originHEAD string
		available  string
		want       string
	}{
		{name: "origin HEAD", originHEAD: "origin/trunk", want: "origin/trunk"},
		{name: "origin main", available: "origin/main", want: "origin/main"},
		{name: "local main", available: "main", want: "main"},
		{
			name:      "origin master",
			available: "origin/master",
			want:      "origin/master",
		},
		{name: "local master", available: "master", want: "master"},
		{name: "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			commandRunner := &patchDefaultBranchRunner{
				root:       root,
				originHEAD: tt.originHEAD,
				available:  tt.available,
			}
			got, err := func() (string, error) {
				var ctx context.Context = context.Background()
				out, err := patchRunner(commandRunner).Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
				if err == nil {
					return strings.TrimSpace(string(out)), nil
				}
				for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
					_, err := patchRunner(commandRunner).Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
					if ctx.Err() != nil {
						return "", ctx.Err()
					}
					if err == nil {
						return candidate, nil
					}
				}
				return "", nil
			}()
			(func(t *testing.T, err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			})(t, err)
			(func(t *testing.T, got, want string, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, got, tt.want,
				"default branch = %q, want %q", got, tt.want,
			)

			for _, call := range commandRunner.calls {
				(func(t *testing.T, got, want bool, format string, args ...any) {
					t.Helper()
					if got != want {
						t.Fatalf(format, args...)
					}
				})(t,
					!strings.Contains(call, "for-each-ref"), true,
					"default branch discovery inspected refs: git %s", call,
				)
				(func(t *testing.T, got, want bool, format string, args ...any) {
					t.Helper()
					if got != want {
						t.Fatalf(format, args...)
					}
				})(t, !strings.Contains(call, "merge-base"),
					true, "default branch discovery inspected refs: git %s",
					call,
				)
				(func(t *testing.T, got, want bool, format string, args ...any) {
					t.Helper()
					if got != want {
						t.Fatalf(format, args...)
					}
				})(t, !strings.Contains(call, "rev-list"),
					true, "default branch discovery inspected refs: git %s",
					call,
				)
			}
		})
	}
}

func TestPatchGetBranchKeepsPathsForAdditionsDeletionsAndRenames(t *testing.T) {
	repo := func() string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, err)
		var args []string = []string{"init", "-q"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		{
			var args []string = []string{"config", "user.email", "test@example.com"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		{
			var args []string = []string{"config", "user.name", "Test"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		return repo
	}()
	var args []string = []string{"branch", "-M", "main"}
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	t.Helper()
	path := filepath.Join(repo, "deleted.txt")
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, os.MkdirAll(filepath.Dir(path), 0o755))
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, os.WriteFile(path, []byte("delete this\n"), 0o644))
	{
		t.Helper()
		path := filepath.Join(repo, "old.go")
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.MkdirAll(filepath.Dir(path), 0o755))
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.WriteFile(path, []byte("package main\n\nfunc example() {}\n"), 0o644))
	}
	{
		var args []string = []string{"add", "."}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		var args []string = []string{"commit", "-m", "base"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		var args []string = []string{"switch", "-c", "feature"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		var args []string = []string{"mv", "old.go", "new.go"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		var args []string = []string{"rm", "deleted.txt"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		t.Helper()
		path := filepath.Join(repo, "added.txt")
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.MkdirAll(filepath.Dir(path), 0o755))
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.WriteFile(path, []byte("new content\n"), 0o644))
	}
	{
		var args []string = []string{"add", "."}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		var args []string = []string{"commit", "-m", "changes"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		t.Helper()
		path := filepath.Join(repo, "untracked.txt")
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.MkdirAll(filepath.Dir(path), 0o755))
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, os.WriteFile(path, []byte("untracked content\n"), 0o644))
	}
	t.Chdir(repo)
	got, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
		directory, err := os.Getwd()
		if err != nil {
			return patchPatch{}, fmt.Errorf("resolve working directory: %w", err)
		}
		{
			if kind != patchUnstaged && kind != patchBranch {
				return patchPatch{}, fmt.Errorf("unknown patch kind %d", kind)
			}
			if err := ctx.Err(); err != nil {
				return patchPatch{}, err
			}
			root, err := func() (string, error) {
				rootBytes, err := patchRunner(patchExecRunner{}).Run(ctx, directory, "rev-parse", "--show-toplevel")
				if err != nil {
					return "", err
				}
				root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
				if err != nil {
					return "", fmt.Errorf("resolve repository root: %w", err)
				}
				return root, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			branch, err := func() (string, error) {
				out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
				if err == nil {
					return strings.TrimSpace(string(out)), nil
				}
				for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
					_, err := patchRunner(patchExecRunner{}).Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
					if ctx.Err() != nil {
						return "", ctx.Err()
					}
					if err == nil {
						return candidate, nil
					}
				}
				return "", nil
			}()
			if err != nil {
				return patchPatch{}, fmt.Errorf("discover default branch: %w", err)
			}
			revisions, readOld, err := func() ([]string, patchSourceReader, error) {
				if kind != patchBranch {
					return nil, func(ctx context.Context, run patchRunner, root, path string) string {
						if path == "" || path == "/dev/null" {
							return ""
						}
						out, err := run.Run(ctx, root, "show", ":"+path)
						if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
							return ""
						}
						var result strings.Builder
						for _, r := range string(out) {
							switch {
							case r == '\n' || r == '\t':
								_, _ = result.WriteRune(r)
							case r == '\r':
								_, _ = result.WriteString(`\r`)
							case r < 0x20 || r == 0x7f:
								fmt.Fprintf(&result, `\x%02x`, r)
							default:
								_, _ = result.WriteRune(r)
							}
						}
						return result.String()
					}, nil
				}
				if branch == "" {
					return nil, nil, errors.New("default branch is unavailable")
				}
				baseBytes, err := patchRunner(patchExecRunner{}).Run(ctx, root, "merge-base", branch, "HEAD")
				if err != nil {
					return nil, nil, fmt.Errorf("find branch point with %s: %w", branch, err)
				}
				base := strings.TrimSpace(string(baseBytes))
				readOld := func(ctx context.Context, run patchRunner, root, path string) string {
					if path == "" || path == "/dev/null" {
						return ""
					}
					out, err := run.Run(ctx, root, "show", base+":"+path)
					if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
						return ""
					}
					var result strings.Builder
					for _, r := range string(out) {
						switch {
						case r == '\n' || r == '\t':
							_, _ = result.WriteRune(r)
						case r == '\r':
							_, _ = result.WriteString(`\r`)
						case r < 0x20 || r == 0x7f:
							fmt.Fprintf(&result, `\x%02x`, r)
						default:
							_, _ = result.WriteRune(r)
						}
					}
					return result.String()
				}
				return []string{base}, readOld, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			raw, err := func() ([]byte, error) {
				args := []string{"-c", "core.quotepath=false", "-c", "diff.external=", "--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--unified=3"}
				args = append(args, revisions...)
				args = append(args, "--")
				return patchRunner(patchExecRunner{}).Run(ctx, root, args...)
			}()
			if err != nil {
				return patchPatch{}, err
			}
			{
				files, err := func() ([]patchFile, error) {
					if len(bytes.TrimSpace(raw)) == 0 {
						return nil, nil
					}
					parsed, err := diff.ParseMultiFileDiff(raw)
					if err != nil {
						return nil, fmt.Errorf("parse git diff: %w", err)
					}
					files := make([]patchFile, 0, len(parsed))
					for _, fd := range parsed {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, err := func() (patchFile, error) {
							oldPath := func() string {
								var path string = fd.OrigName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							newPath := func() string {
								var path string = fd.NewName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							display := newPath
							if display == "" || display == "/dev/null" {
								display = oldPath
							}
							file := patchFile{OldPath: oldPath, NewPath: newPath, DisplayPath: func() string {
								var value string = display
								return strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range value {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`)
							}(), Metadata: func() []string {
								var values []string = fd.Extended
								result := make([]string, len(values))
								for index, value := range values {
									result[index] = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range value {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
								}
								return result
							}()}
							file.OldSource = readOld(ctx, patchExecRunner{}, root, oldPath)
							file.NewSource = func() string {
								if newPath == "" || newPath == "/dev/null" {
									return ""
								}
								full := filepath.Join(root, filepath.FromSlash(newPath))
								info, err := os.Lstat(full)
								if err != nil || !info.Mode().IsRegular() || info.Size() > patchMaxFileBytes {
									return ""
								}
								out, err := os.ReadFile(full)
								if err != nil || bytes.IndexByte(out, 0) >= 0 {
									return ""
								}
								var result strings.Builder
								for _, r := range string(out) {
									switch {
									case r == '\n' || r == '\t':
										_, _ = result.WriteRune(r)
									case r == '\r':
										_, _ = result.WriteString(`\r`)
									case r < 0x20 || r == 0x7f:
										fmt.Fprintf(&result, `\x%02x`, r)
									default:
										_, _ = result.WriteRune(r)
									}
								}
								return result.String()
							}()
							for _, h := range fd.Hunks {
								lines, err := func() ([]patchLine, error) {
									var (
										oldLine, newLine int32  = h.OrigStartLine, h.NewStartLine
										body             []byte = h.Body
									)
									rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
									if len(body) == 0 {
										return []patchLine{}, nil
									}
									lines := make([]patchLine, 0, len(rawLines))
									for _, raw := range rawLines {
										if len(raw) == 0 {
											return nil, errors.New("malformed empty diff line")
										}
										text := func() string {
											var value string = string(raw[1:])
											return strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range value {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`)
										}()
										switch raw[0] {
										case ' ':
											lines = append(lines, patchLine{Kind: patchContext, Text: text, OldNumber: patchLineNumber(oldLine), NewNumber: patchLineNumber(newLine)})
											oldLine++
											newLine++
										case '+':
											lines = append(lines, patchLine{Kind: patchAddition, Text: text, NewNumber: patchLineNumber(newLine)})
											newLine++
										case '-':
											lines = append(lines, patchLine{Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(oldLine)})
											oldLine++
										case '\\':
										default:
											return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
										}
									}
									return lines, nil
								}()
								if err != nil {
									return patchFile{}, fmt.Errorf("%s: %w", display, err)
								}
								file.Hunks = append(file.Hunks, patchHunk{Header: func() string {
									header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
									if h.Section != "" {
										header += " " + h.Section
									}
									return header
								}(), Lines: lines})
							}
							return file, nil
						}()
						if err != nil {
							return nil, err
						}
						files = append(files, file)
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				untracked, err := func() ([]patchFile, error) {
					out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
					if err != nil {
						return nil, err
					}
					var files []patchFile
					for _, path := range func() []string {
						var paths []string
						for path := range bytes.SplitSeq(out, []byte{0}) {
							if len(path) > 0 {
								paths = append(paths, string(path))
							}
						}
						return paths
					}() {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, include, err := func() (patchFile, bool, error) {
							full := filepath.Join(root, filepath.FromSlash(path))
							info, err := os.Lstat(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
							}
							if info.Mode()&os.ModeSymlink != 0 {
								target, err := os.Readlink(full)
								if err != nil {
									return patchFile{}, false, fmt.Errorf("read symlink %q: %w", path, err)
								}
								return func() patchFile {
									var content string = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range target {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
									sourceLines := func() []string {
										var content string = content
										content = strings.TrimSuffix(content, "\n")
										if content == "" {
											return nil
										}
										return strings.Split(content, "\n")
									}()
									lines := make([]patchLine, 0, len(sourceLines))
									for i, line := range sourceLines {
										lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
									}
									return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range path {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
								}(), true, nil
							}
							if !info.Mode().IsRegular() {
								return patchFile{}, false, nil
							}
							if info.Size() > patchMaxFileBytes {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked file", "content omitted: file exceeds 2 MiB"}}, true, nil
							}
							content, err := os.ReadFile(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("read untracked %q: %w", path, err)
							}
							if bytes.IndexByte(content, 0) >= 0 {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked binary file"}}, true, nil
							}
							return func() patchFile {
								var content string = func() string {
									var result strings.Builder
									for _, r := range string(content) {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}()
								sourceLines := func() []string {
									var content string = content
									content = strings.TrimSuffix(content, "\n")
									if content == "" {
										return nil
									}
									return strings.Split(content, "\n")
								}()
								lines := make([]patchLine, 0, len(sourceLines))
								for i, line := range sourceLines {
									lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
								}
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
							}(), true, nil
						}()
						if err != nil {
							return nil, err
						}
						if include {
							files = append(files, file)
						}
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				files = append(files, untracked...)
				slices.SortStableFunc(files, func(a, b patchFile) int { return cmp.Compare(a.DisplayPath, b.DisplayPath) })
				if err := ctx.Err(); err != nil {
					return patchPatch{}, err
				}
				return patchPatch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
			}
		}
	})(context.Background(), patchBranch)
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)

	want := []struct {
		display string
		old     string
		new     string
	}{
		{"added.txt", "", "added.txt"},
		{"deleted.txt", "deleted.txt", ""},
		{"new.go", "old.go", "new.go"},
		{"untracked.txt", "", "untracked.txt"},
	}
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(got.Files), len(want), "files = %#v",
		got.Files,
	)

	for i, paths := range want {
		file := got.Files[i]
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Errorf(format, args...)
			}
		})(
			t,
			file.DisplayPath == paths.display &&
				file.OldPath == paths.old && file.NewPath == paths.new,
			true,
			"file %d paths = %#v, want %#v",
			i,
			file,
			paths,
		)
	}
}

type patchDefaultBranchRunner struct {
	root       string
	originHEAD string
	available  string
	calls      []string
}

func (r *patchDefaultBranchRunner) Run(
	_ context.Context,
	_ string,
	args ...string,
) ([]byte, error) {
	call := strings.Join(args, " ")
	r.calls = append(r.calls, call)
	switch {
	case call == "rev-parse --show-toplevel":
		return []byte(r.root + "\n"), nil
	case call == "symbolic-ref --quiet --short refs/remotes/origin/HEAD" &&
		r.originHEAD != "":
		return []byte(r.originHEAD + "\n"), nil
	case call == "rev-parse --verify --quiet "+r.available+"^{commit}" &&
		r.available != "":
		return []byte("commit\n"), nil
	default:
		return nil, exec.ErrNotFound
	}
}

func TestPatchGetDoesNotFollowUntrackedSymlink(t *testing.T) {
	repo := func() string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, err)
		var args []string = []string{"init", "-q"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		{
			var args []string = []string{"config", "user.email", "test@example.com"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		{
			var args []string = []string{"config", "user.name", "Test"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		return repo
	}()
	outside := filepath.Join(t.TempDir(), "secret")
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, os.WriteFile(outside, []byte("do not read"), 0o600))
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, os.Symlink(outside, filepath.Join(repo, "link")))

	t.Chdir(repo)
	got, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
		directory, err := os.Getwd()
		if err != nil {
			return patchPatch{}, fmt.Errorf("resolve working directory: %w", err)
		}
		{
			if kind != patchUnstaged && kind != patchBranch {
				return patchPatch{}, fmt.Errorf("unknown patch kind %d", kind)
			}
			if err := ctx.Err(); err != nil {
				return patchPatch{}, err
			}
			root, err := func() (string, error) {
				rootBytes, err := patchRunner(patchExecRunner{}).Run(ctx, directory, "rev-parse", "--show-toplevel")
				if err != nil {
					return "", err
				}
				root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
				if err != nil {
					return "", fmt.Errorf("resolve repository root: %w", err)
				}
				return root, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			branch, err := func() (string, error) {
				out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
				if err == nil {
					return strings.TrimSpace(string(out)), nil
				}
				for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
					_, err := patchRunner(patchExecRunner{}).Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
					if ctx.Err() != nil {
						return "", ctx.Err()
					}
					if err == nil {
						return candidate, nil
					}
				}
				return "", nil
			}()
			if err != nil {
				return patchPatch{}, fmt.Errorf("discover default branch: %w", err)
			}
			revisions, readOld, err := func() ([]string, patchSourceReader, error) {
				if kind != patchBranch {
					return nil, func(ctx context.Context, run patchRunner, root, path string) string {
						if path == "" || path == "/dev/null" {
							return ""
						}
						out, err := run.Run(ctx, root, "show", ":"+path)
						if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
							return ""
						}
						var result strings.Builder
						for _, r := range string(out) {
							switch {
							case r == '\n' || r == '\t':
								_, _ = result.WriteRune(r)
							case r == '\r':
								_, _ = result.WriteString(`\r`)
							case r < 0x20 || r == 0x7f:
								fmt.Fprintf(&result, `\x%02x`, r)
							default:
								_, _ = result.WriteRune(r)
							}
						}
						return result.String()
					}, nil
				}
				if branch == "" {
					return nil, nil, errors.New("default branch is unavailable")
				}
				baseBytes, err := patchRunner(patchExecRunner{}).Run(ctx, root, "merge-base", branch, "HEAD")
				if err != nil {
					return nil, nil, fmt.Errorf("find branch point with %s: %w", branch, err)
				}
				base := strings.TrimSpace(string(baseBytes))
				readOld := func(ctx context.Context, run patchRunner, root, path string) string {
					if path == "" || path == "/dev/null" {
						return ""
					}
					out, err := run.Run(ctx, root, "show", base+":"+path)
					if err != nil || len(out) > patchMaxFileBytes || bytes.IndexByte(out, 0) >= 0 {
						return ""
					}
					var result strings.Builder
					for _, r := range string(out) {
						switch {
						case r == '\n' || r == '\t':
							_, _ = result.WriteRune(r)
						case r == '\r':
							_, _ = result.WriteString(`\r`)
						case r < 0x20 || r == 0x7f:
							fmt.Fprintf(&result, `\x%02x`, r)
						default:
							_, _ = result.WriteRune(r)
						}
					}
					return result.String()
				}
				return []string{base}, readOld, nil
			}()
			if err != nil {
				return patchPatch{}, err
			}
			raw, err := func() ([]byte, error) {
				args := []string{"-c", "core.quotepath=false", "-c", "diff.external=", "--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--unified=3"}
				args = append(args, revisions...)
				args = append(args, "--")
				return patchRunner(patchExecRunner{}).Run(ctx, root, args...)
			}()
			if err != nil {
				return patchPatch{}, err
			}
			{
				files, err := func() ([]patchFile, error) {
					if len(bytes.TrimSpace(raw)) == 0 {
						return nil, nil
					}
					parsed, err := diff.ParseMultiFileDiff(raw)
					if err != nil {
						return nil, fmt.Errorf("parse git diff: %w", err)
					}
					files := make([]patchFile, 0, len(parsed))
					for _, fd := range parsed {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, err := func() (patchFile, error) {
							oldPath := func() string {
								var path string = fd.OrigName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							newPath := func() string {
								var path string = fd.NewName
								path = strings.TrimSpace(path)
								path = strings.TrimPrefix(path, "a/")
								path = strings.TrimPrefix(path, "b/")
								if path == "/dev/null" {
									return ""
								}
								return path
							}()
							display := newPath
							if display == "" || display == "/dev/null" {
								display = oldPath
							}
							file := patchFile{OldPath: oldPath, NewPath: newPath, DisplayPath: func() string {
								var value string = display
								return strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range value {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`)
							}(), Metadata: func() []string {
								var values []string = fd.Extended
								result := make([]string, len(values))
								for index, value := range values {
									result[index] = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range value {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
								}
								return result
							}()}
							file.OldSource = readOld(ctx, patchExecRunner{}, root, oldPath)
							file.NewSource = func() string {
								if newPath == "" || newPath == "/dev/null" {
									return ""
								}
								full := filepath.Join(root, filepath.FromSlash(newPath))
								info, err := os.Lstat(full)
								if err != nil || !info.Mode().IsRegular() || info.Size() > patchMaxFileBytes {
									return ""
								}
								out, err := os.ReadFile(full)
								if err != nil || bytes.IndexByte(out, 0) >= 0 {
									return ""
								}
								var result strings.Builder
								for _, r := range string(out) {
									switch {
									case r == '\n' || r == '\t':
										_, _ = result.WriteRune(r)
									case r == '\r':
										_, _ = result.WriteString(`\r`)
									case r < 0x20 || r == 0x7f:
										fmt.Fprintf(&result, `\x%02x`, r)
									default:
										_, _ = result.WriteRune(r)
									}
								}
								return result.String()
							}()
							for _, h := range fd.Hunks {
								lines, err := func() ([]patchLine, error) {
									var (
										oldLine, newLine int32  = h.OrigStartLine, h.NewStartLine
										body             []byte = h.Body
									)
									rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
									if len(body) == 0 {
										return []patchLine{}, nil
									}
									lines := make([]patchLine, 0, len(rawLines))
									for _, raw := range rawLines {
										if len(raw) == 0 {
											return nil, errors.New("malformed empty diff line")
										}
										text := func() string {
											var value string = string(raw[1:])
											return strings.ReplaceAll(func() string {
												var result strings.Builder
												for _, r := range value {
													switch {
													case r == '\n' || r == '\t':
														_, _ = result.WriteRune(r)
													case r == '\r':
														_, _ = result.WriteString(`\r`)
													case r < 0x20 || r == 0x7f:
														fmt.Fprintf(&result, `\x%02x`, r)
													default:
														_, _ = result.WriteRune(r)
													}
												}
												return result.String()
											}(), "\n", `\n`)
										}()
										switch raw[0] {
										case ' ':
											lines = append(lines, patchLine{Kind: patchContext, Text: text, OldNumber: patchLineNumber(oldLine), NewNumber: patchLineNumber(newLine)})
											oldLine++
											newLine++
										case '+':
											lines = append(lines, patchLine{Kind: patchAddition, Text: text, NewNumber: patchLineNumber(newLine)})
											newLine++
										case '-':
											lines = append(lines, patchLine{Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(oldLine)})
											oldLine++
										case '\\':
										default:
											return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
										}
									}
									return lines, nil
								}()
								if err != nil {
									return patchFile{}, fmt.Errorf("%s: %w", display, err)
								}
								file.Hunks = append(file.Hunks, patchHunk{Header: func() string {
									header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
									if h.Section != "" {
										header += " " + h.Section
									}
									return header
								}(), Lines: lines})
							}
							return file, nil
						}()
						if err != nil {
							return nil, err
						}
						files = append(files, file)
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				untracked, err := func() ([]patchFile, error) {
					out, err := patchRunner(patchExecRunner{}).Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
					if err != nil {
						return nil, err
					}
					var files []patchFile
					for _, path := range func() []string {
						var paths []string
						for path := range bytes.SplitSeq(out, []byte{0}) {
							if len(path) > 0 {
								paths = append(paths, string(path))
							}
						}
						return paths
					}() {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
						file, include, err := func() (patchFile, bool, error) {
							full := filepath.Join(root, filepath.FromSlash(path))
							info, err := os.Lstat(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
							}
							if info.Mode()&os.ModeSymlink != 0 {
								target, err := os.Readlink(full)
								if err != nil {
									return patchFile{}, false, fmt.Errorf("read symlink %q: %w", path, err)
								}
								return func() patchFile {
									var content string = strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range target {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`)
									sourceLines := func() []string {
										var content string = content
										content = strings.TrimSuffix(content, "\n")
										if content == "" {
											return nil
										}
										return strings.Split(content, "\n")
									}()
									lines := make([]patchLine, 0, len(sourceLines))
									for i, line := range sourceLines {
										lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
									}
									return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
										var result strings.Builder
										for _, r := range path {
											switch {
											case r == '\n' || r == '\t':
												_, _ = result.WriteRune(r)
											case r == '\r':
												_, _ = result.WriteString(`\r`)
											case r < 0x20 || r == 0x7f:
												fmt.Fprintf(&result, `\x%02x`, r)
											default:
												_, _ = result.WriteRune(r)
											}
										}
										return result.String()
									}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
								}(), true, nil
							}
							if !info.Mode().IsRegular() {
								return patchFile{}, false, nil
							}
							if info.Size() > patchMaxFileBytes {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked file", "content omitted: file exceeds 2 MiB"}}, true, nil
							}
							content, err := os.ReadFile(full)
							if err != nil {
								return patchFile{}, false, fmt.Errorf("read untracked %q: %w", path, err)
							}
							if bytes.IndexByte(content, 0) >= 0 {
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), Metadata: []string{"untracked binary file"}}, true, nil
							}
							return func() patchFile {
								var content string = func() string {
									var result strings.Builder
									for _, r := range string(content) {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}()
								sourceLines := func() []string {
									var content string = content
									content = strings.TrimSuffix(content, "\n")
									if content == "" {
										return nil
									}
									return strings.Split(content, "\n")
								}()
								lines := make([]patchLine, 0, len(sourceLines))
								for i, line := range sourceLines {
									lines = append(lines, patchLine{Kind: patchAddition, Text: line, NewNumber: patchLineNumber(i + 1)})
								}
								return patchFile{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
									var result strings.Builder
									for _, r := range path {
										switch {
										case r == '\n' || r == '\t':
											_, _ = result.WriteRune(r)
										case r == '\r':
											_, _ = result.WriteString(`\r`)
										case r < 0x20 || r == 0x7f:
											fmt.Fprintf(&result, `\x%02x`, r)
										default:
											_, _ = result.WriteRune(r)
										}
									}
									return result.String()
								}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []patchHunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
							}(), true, nil
						}()
						if err != nil {
							return nil, err
						}
						if include {
							files = append(files, file)
						}
					}
					return files, nil
				}()
				if err != nil {
					return patchPatch{}, err
				}
				files = append(files, untracked...)
				slices.SortStableFunc(files, func(a, b patchFile) int { return cmp.Compare(a.DisplayPath, b.DisplayPath) })
				if err := ctx.Err(); err != nil {
					return patchPatch{}, err
				}
				return patchPatch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
			}
		}
	})(context.Background(), patchUnstaged)
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(got.Files), 1, "files = %d, want 1",
		len(got.Files),
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got.Files[0].NewSource, outside,
		"symlink source = %q, want link target %q", got.Files[0].NewSource,
		outside,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got.Files[0].OldPath, "", "symlink paths = %#v",
		got.Files[0],
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got.Files[0].NewPath, "link",
		"symlink paths = %#v", got.Files[0],
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		!strings.Contains(got.Files[0].NewSource, "do not read"), true, "%s",
		"symlink target contents were read",
	)
}

func TestPatchParseHunkBodyLineNumbers(t *testing.T) {
	lines, err := func() ([]patchLine, error) {
		var (
			oldLine, newLine int32 = 10,
				20
			body []byte = []byte(" context\n-old\n+new\n same\n\\ No newline at end of file\n")
		)
		rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
		if len(body) == 0 {
			return []patchLine{}, nil
		}
		lines := make([]patchLine, 0, len(rawLines))
		for _, raw := range rawLines {
			if len(raw) == 0 {
				return nil, errors.New("malformed empty diff line")
			}
			text := func() string {
				var value string = string(raw[1:])
				return strings.ReplaceAll(func() string {
					var result strings.Builder
					for _, r := range value {
						switch {
						case r == '\n' || r == '\t':
							_, _ = result.WriteRune(r)
						case r == '\r':
							_, _ = result.WriteString(`\r`)
						case r < 0x20 || r == 0x7f:
							fmt.Fprintf(&result, `\x%02x`, r)
						default:
							_, _ = result.WriteRune(r)
						}
					}
					return result.String()
				}(), "\n", `\n`)
			}()
			switch raw[0] {
			case ' ':
				lines = append(lines, patchLine{Kind: patchContext, Text: text, OldNumber: patchLineNumber(oldLine), NewNumber: patchLineNumber(newLine)})
				oldLine++
				newLine++
			case '+':
				lines = append(lines, patchLine{Kind: patchAddition, Text: text, NewNumber: patchLineNumber(newLine)})
				newLine++
			case '-':
				lines = append(lines, patchLine{Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(oldLine)})
				oldLine++
			case '\\':
			default:
				return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
			}
		}
		return lines, nil
	}()
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)

	want := []patchLine{
		{Kind: patchContext, Text: "context", OldNumber: 10, NewNumber: 20},
		{Kind: patchDeletion, Text: "old", OldNumber: 11},
		{Kind: patchAddition, Text: "new", NewNumber: 21},
		{Kind: patchContext, Text: "same", OldNumber: 12, NewNumber: 22},
	}
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(lines), len(want), "lines = %#v", lines)

	for i := range want {
		(func(t *testing.T, got, want patchLine, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, lines[i], want[i], "line %d = %#v, want %#v",
			i, lines[i], want[i],
		)
	}
}

func TestPatchParseHunkBodyEscapesTerminalControls(t *testing.T) {
	lines, err := func() ([]patchLine, error) {
		var (
			oldLine, newLine int32  = 1, 1
			body             []byte = []byte("+hello\x1b[2J\n")
		)
		rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
		if len(body) == 0 {
			return []patchLine{}, nil
		}
		lines := make([]patchLine, 0, len(rawLines))
		for _, raw := range rawLines {
			if len(raw) == 0 {
				return nil, errors.New("malformed empty diff line")
			}
			text := func() string {
				var value string = string(raw[1:])
				return strings.ReplaceAll(func() string {
					var result strings.Builder
					for _, r := range value {
						switch {
						case r == '\n' || r == '\t':
							_, _ = result.WriteRune(r)
						case r == '\r':
							_, _ = result.WriteString(`\r`)
						case r < 0x20 || r == 0x7f:
							fmt.Fprintf(&result, `\x%02x`, r)
						default:
							_, _ = result.WriteRune(r)
						}
					}
					return result.String()
				}(), "\n", `\n`)
			}()
			switch raw[0] {
			case ' ':
				lines = append(lines, patchLine{Kind: patchContext, Text: text, OldNumber: patchLineNumber(oldLine), NewNumber: patchLineNumber(newLine)})
				oldLine++
				newLine++
			case '+':
				lines = append(lines, patchLine{Kind: patchAddition, Text: text, NewNumber: patchLineNumber(newLine)})
				newLine++
			case '-':
				lines = append(lines, patchLine{Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(oldLine)})
				oldLine++
			case '\\':
			default:
				return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
			}
		}
		return lines, nil
	}()
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, lines[0].Text, `hello\x1b[2J`, "text = %q",
		lines[0].Text,
	)
}

func FuzzPatchParseHunkBody(f *testing.F) {
	f.Add([]byte("+hello\n-world\n"))
	f.Add([]byte(" context\n"))
	f.Fuzz(func(_ *testing.T, body []byte) {
		_, _ = func() ([]patchLine, error) {
			var oldLine, newLine int32 = 1, 1
			rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
			if len(body) == 0 {
				return []patchLine{}, nil
			}
			lines := make([]patchLine, 0, len(rawLines))
			for _, raw := range rawLines {
				if len(raw) == 0 {
					return nil, errors.New("malformed empty diff line")
				}
				text := func() string {
					var value string = string(raw[1:])
					return strings.ReplaceAll(func() string {
						var result strings.Builder
						for _, r := range value {
							switch {
							case r == '\n' || r == '\t':
								_, _ = result.WriteRune(r)
							case r == '\r':
								_, _ = result.WriteString(`\r`)
							case r < 0x20 || r == 0x7f:
								fmt.Fprintf(&result, `\x%02x`, r)
							default:
								_, _ = result.WriteRune(r)
							}
						}
						return result.String()
					}(), "\n", `\n`)
				}()
				switch raw[0] {
				case ' ':
					lines = append(lines, patchLine{Kind: patchContext, Text: text, OldNumber: patchLineNumber(oldLine), NewNumber: patchLineNumber(newLine)})
					oldLine++
					newLine++
				case '+':
					lines = append(lines, patchLine{Kind: patchAddition, Text: text, NewNumber: patchLineNumber(newLine)})
					newLine++
				case '-':
					lines = append(lines, patchLine{Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(oldLine)})
					oldLine++
				case '\\':
				default:
					return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
				}
			}
			return lines, nil
		}()
	})
}
