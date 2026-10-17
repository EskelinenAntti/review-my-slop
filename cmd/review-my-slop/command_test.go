package main

import (
	bytes "bytes"
	cmp "cmp"
	context "context"
	json "encoding/json"
	errors "errors"
	fmt "fmt"
	io "io"
	"io/fs"
	os "os"
	exec "os/exec"
	filepath "path/filepath"
	slices "slices"
	strings "strings"
	testing "testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	quick "github.com/alecthomas/chroma/v2/quick"
	ansi "github.com/charmbracelet/x/ansi"
	term "github.com/charmbracelet/x/term"
	diff "github.com/sourcegraph/go-diff/diff"
)

func TestMainRunCommentsPrintsAndConsumesCurrentRepositoryFeedback(t *testing.T) {
	repo := (func(t *testing.T) string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, err)

		cmd := exec.Command("git", "init", "-q")
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git init: %v\n%s", err, out)
		}
		return repo
	})(t)
	nested := filepath.Join(repo, "nested")
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, os.Mkdir(nested, 0o755))

	t.Chdir(nested)
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	store, err := (func() (commentsStore, error) {
		path, err := (func() (string, error) {
			data, err := (func() (string, error) {
				var fallback string = filepath.Join(".local", "share")
				if root := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(root) {
					return filepath.Join(root, commentsAppName), nil
				}
				home, err := os.UserHomeDir()
				if err != nil {
					return "", fmt.Errorf("resolve user home directory: %w", err)
				}
				return filepath.Join(home, fallback, commentsAppName), nil
			})()
			if err != nil {
				return "", err
			}
			return filepath.Join(data, "comments.db"), nil
		})()
		if err != nil {
			return commentsStore{}, err
		}
		return commentsStore{Path: path}, nil
	})()
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)

	comment := commentsComment{
		Repository: repo,
		Anchor: commentsAnchor{
			FilePath: "main.go",
			NewStart: 3,
			NewEnd:   3,
		},
		Body: "Check this error.",
	}
	if _, err := store.Add(comment); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, (func(ctx context.Context, output io.Writer) error {
		commentStore, err := (func() (commentsStore, error) {
			path, err := (func() (string, error) {
				data, err := (func() (string, error) {
					var fallback string = filepath.Join(".local", "share")
					if root := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(root) {
						return filepath.Join(root, commentsAppName), nil
					}
					home, err := os.UserHomeDir()
					if err != nil {
						return "", fmt.Errorf("resolve user home directory: %w", err)
					}
					return filepath.Join(home, fallback, commentsAppName), nil
				})()
				if err != nil {
					return "", err
				}
				return filepath.Join(data, "comments.db"), nil
			})()
			if err != nil {
				return commentsStore{}, err
			}
			return commentsStore{Path: path}, nil
		})()
		if err != nil {
			return err
		}
		currentPatch, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
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
		if err != nil {
			return err
		}
		return commentStore.WritePending(output, currentPatch.Root)
	})(context.Background(), &output))
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		strings.Contains(output.String(), "Check this error."), true,
		"unexpected output:\n%s", output.String(),
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		strings.HasPrefix(
			output.String(),
			"New comments since last run:\n",
		), true,
		"unexpected output heading:\n%s", output.String(),
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !strings.Contains(output.String(), "batch"), true,
		"output exposes internal batches:\n%s", output.String(),
	)

	var empty bytes.Buffer
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, (func(ctx context.Context, output io.Writer) error {
		commentStore, err := (func() (commentsStore, error) {
			path, err := (func() (string, error) {
				data, err := (func() (string, error) {
					var fallback string = filepath.Join(".local", "share")
					if root := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(root) {
						return filepath.Join(root, commentsAppName), nil
					}
					home, err := os.UserHomeDir()
					if err != nil {
						return "", fmt.Errorf("resolve user home directory: %w", err)
					}
					return filepath.Join(home, fallback, commentsAppName), nil
				})()
				if err != nil {
					return "", err
				}
				return filepath.Join(data, "comments.db"), nil
			})()
			if err != nil {
				return commentsStore{}, err
			}
			return commentsStore{Path: path}, nil
		})()
		if err != nil {
			return err
		}
		currentPatch, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
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
		if err != nil {
			return err
		}
		return commentStore.WritePending(output, currentPatch.Root)
	})(context.Background(), &empty))
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(
		t, strings.TrimSpace(empty.String()), "No pending review comments.",
		"second output = %q", empty.String())

	info, err := os.Stat(filepath.Join(data, "review-my-slop", "comments.db"))
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want fs.FileMode, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, info.Mode().Perm(), 0o600, "database mode = %o",
		info.Mode().Perm(),
	)
}

func TestMainRunCommentsPreservesFeedbackWhenOutputFails(t *testing.T) {
	repo := (func(t *testing.T) string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, err)

		cmd := exec.Command("git", "init", "-q")
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git init: %v\n%s", err, out)
		}
		return repo
	})(t)
	t.Chdir(repo)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	store, err := (func() (commentsStore, error) {
		path, err := (func() (string, error) {
			data, err := (func() (string, error) {
				var fallback string = filepath.Join(".local", "share")
				if root := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(root) {
					return filepath.Join(root, commentsAppName), nil
				}
				home, err := os.UserHomeDir()
				if err != nil {
					return "", fmt.Errorf("resolve user home directory: %w", err)
				}
				return filepath.Join(home, fallback, commentsAppName), nil
			})()
			if err != nil {
				return "", err
			}
			return filepath.Join(data, "comments.db"), nil
		})()
		if err != nil {
			return commentsStore{}, err
		}
		return commentsStore{Path: path}, nil
	})()
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)

	comment := commentsComment{
		Repository: repo,
		Anchor:     commentsAnchor{FilePath: "main.go", NewStart: 1},
		Body:       "Preserve me.",
	}
	if _, err := store.Add(comment); err != nil {
		t.Fatal(err)
	}

	if err := (func(ctx context.Context, output io.Writer) error {
		commentStore, err := (func() (commentsStore, error) {
			path, err := (func() (string, error) {
				data, err := (func() (string, error) {
					var fallback string = filepath.Join(".local", "share")
					if root := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(root) {
						return filepath.Join(root, commentsAppName), nil
					}
					home, err := os.UserHomeDir()
					if err != nil {
						return "", fmt.Errorf("resolve user home directory: %w", err)
					}
					return filepath.Join(home, fallback, commentsAppName), nil
				})()
				if err != nil {
					return "", err
				}
				return filepath.Join(data, "comments.db"), nil
			})()
			if err != nil {
				return commentsStore{}, err
			}
			return commentsStore{Path: path}, nil
		})()
		if err != nil {
			return err
		}
		currentPatch, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
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
		if err != nil {
			return err
		}
		return commentStore.WritePending(output, currentPatch.Root)
	})(context.Background(), mainFailingWriter{}); err == nil {
		t.Fatal("output failure was ignored")
	}
	pending, err := store.List(repo)
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
	})(t, len(pending), 1, "pending comments = %d, want 1",
		len(pending),
	)
}

func TestMainRunRejectsUnknownSubcommand(t *testing.T) {
	err := (func(ctx context.Context, args []string, output io.Writer) error {
		if len(args) == 0 {
			return (func(ctx context.Context) error {
				commentStore, err := (func() (commentsStore, error) {
					path, err := (func() (string, error) {
						data, err := (func() (string, error) {
							var fallback string = filepath.Join(".local", "share")
							if root := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(root) {
								return filepath.Join(root, commentsAppName), nil
							}
							home, err := os.UserHomeDir()
							if err != nil {
								return "", fmt.Errorf("resolve user home directory: %w", err)
							}
							return filepath.Join(home, fallback, commentsAppName), nil
						})()
						if err != nil {
							return "", err
						}
						return filepath.Join(data, "comments.db"), nil
					})()
					if err != nil {
						return commentsStore{}, err
					}
					return commentsStore{Path: path}, nil
				})()
				if err != nil {
					return err
				}
				currentPatch, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
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
				if err != nil {
					return err
				}
				return (func(
					ctx context.Context,
					currentPatch patchPatch,
					inlineLocalCommentStore1 commentsStore,
					refresh func(context.Context, patchKind) (patchPatch, error),
				) error {
					return (func(
						ctx context.Context,
						currentPatch patchPatch,
						store appCommentStore, refresh func(context.Context, patchKind) (patchPatch, error),
						terminalSize func() appSize, launch func(appModel, ...tea.ProgramOption) error,
					) error {
						pending, err := store.List(currentPatch.Root)
						if err != nil {
							return err
						}
						dimensions := terminalSize()
						m, err := func() (appModel, error) {
							preferences, err := (func() (settingsPreferences, error) {
								path, err := (func() (string, error) {
									config, err := os.UserConfigDir()
									if err != nil {
										return "", fmt.Errorf("resolve UI settings directory: %w", err)
									}
									return filepath.Join(config, "review-my-slop", "ui.json"), nil
								})()
								if err != nil {
									return settingsPreferences{}, err
								}
								return (func(path string) (settingsPreferences, error) {
									data, err := os.ReadFile(path)
									if errors.Is(err, os.ErrNotExist) {
										return settingsPreferences{}, nil
									}
									if err != nil {
										return settingsPreferences{}, fmt.Errorf("read UI settings: %w", err)
									}
									var settings settingsPreferences
									if err := json.Unmarshal(data, &settings); err != nil {
										return settingsPreferences{}, fmt.Errorf("decode UI settings: %w", err)
									}
									return settings, nil
								})(path)
							})()
							if err != nil {
								return appModel{}, err
							}
							m := func() appModel {
								var layout appInitialLayout = appInitialLayout{SideBySide: preferences.SideBySide, SaveSideBySide: func(enabled bool) error {
									return (func(preferences settingsPreferences) error {
										path, err := (func() (string, error) {
											config, err := os.UserConfigDir()
											if err != nil {
												return "", fmt.Errorf("resolve UI settings directory: %w", err)
											}
											return filepath.Join(config, "review-my-slop", "ui.json"), nil
										})()
										if err != nil {
											return err
										}
										return (func(path string, preferences settingsPreferences) error {
											if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
												return fmt.Errorf("create UI settings directory: %w", err)
											}
											data, err := json.Marshal(preferences)
											if err != nil {
												return fmt.Errorf("encode UI settings: %w", err)
											}
											data = append(data, '\n')
											temporary, err := os.CreateTemp(filepath.Dir(path), "ui-*.tmp")
											if err != nil {
												return fmt.Errorf("create UI settings file: %w", err)
											}
											temporaryPath := temporary.Name()
											defer os.Remove(temporaryPath)
											if err := temporary.Chmod(0o600); err != nil {
												_ = temporary.Close()
												return fmt.Errorf("secure UI settings file: %w", err)
											}
											if _, err := temporary.Write(data); err != nil {
												_ = temporary.Close()
												return fmt.Errorf("write UI settings: %w", err)
											}
											if err := temporary.Close(); err != nil {
												return fmt.Errorf("close UI settings: %w", err)
											}
											if err := os.Rename(temporaryPath, path); err != nil {
												return fmt.Errorf("replace UI settings: %w", err)
											}
											return nil
										})(path, preferences)
									})(settingsPreferences{SideBySide: enabled})
								}, size: dimensions}
								inlineLocalSize1 := layout.size
								if inlineLocalSize1.Width <= 0 || inlineLocalSize1.Height <= 0 {
									inlineLocalSize1 = appDefaultSize
								}
								m := appModel{ctx: context.Background(), currentPatch: currentPatch, comments: appCommentState{items: pending}, commentView: (func(items []commentsComment) *commentscreenView {
									v := &commentscreenView{width: 80, height: 30}
									v.Update(items)
									return v
								})(pending), edit: appCommentEdit{index: -1}, width: inlineLocalSize1.Width, height: inlineLocalSize1.Height, save: func(comment commentsComment, current patchPatch) (commentsComment, error) {
									comment.Repository = current.Root
									if comment.ID != "" {
										if err := store.Update(comment); err != nil {
											return commentsComment{}, err
										}
										return comment, nil
									}
									return store.Add(comment)
								}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: currentPatch.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
									m := keymapMatcher{sequences: make(map[string]keymapSequence, len(sequences))}
									for _, sequence := range sequences {
										sequence.Keys = append([]string(nil), sequence.Keys...)
										m.sequences[sequence.Prefix] = sequence
									}
									return m
								})(keymapSequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymapSequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymapSequence{Prefix: "[", Keys: []string{"f"}}, keymapSequence{Prefix: "]", Keys: []string{"f"}}, keymapSequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
								m.diffView = (func(p patchPatch, options diffscreenOptions) *diffscreenView {
									v := &diffscreenView{options: options, width: 80, height: 30}
									v.view = func() *diffscreenDiffView {
										{
											var options diffscreenOptions = diffscreenOptions{SideBySide: v.options.SideBySide && v.width >= 100, Dark: v.options.Dark}
											v := &diffscreenDiffView{patch: p, split: options.SideBySide, dark: options.Dark}
											for fileIndex, file := range p.Files {
												var file patchFile = file
												v.rows = append(v.rows, diffscreenEntry{kind: diffscreenFileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
												for _, metadata := range file.Metadata {
													v.rows = append(v.rows, diffscreenEntry{kind: diffscreenMetadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
												}
												highlighted := func() diffscreenPair {
													var file *patchFile = &file
													theme := diffscreenLightTheme
													if v.dark {
														theme = diffscreenDarkTheme
													}
													return diffscreenPair{Old: func() []string {
														var filename, source, theme string = file.OldPath, file.OldSource, theme
														if source == "" {
															return nil
														}
														var buf bytes.Buffer
														highlightErr := quick.Highlight(&buf, source, filename, "terminal16m", theme)
														if highlightErr != nil {
															return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
														}
														rendered, err := io.ReadAll(&buf)
														if err != nil {
															return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
														}
														return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
													}(), New: func() []string {
														var filename, source, theme string = file.NewPath, file.NewSource, theme
														if source == "" {
															return nil
														}
														var buf bytes.Buffer
														highlightErr := quick.Highlight(&buf, source, filename, "terminal16m", theme)
														if highlightErr != nil {
															return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
														}
														rendered, err := io.ReadAll(&buf)
														if err != nil {
															return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
														}
														return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
													}()}
												}()
												for hunkIndex, hunk := range file.Hunks {
													v.rows = append(v.rows, diffscreenEntry{kind: diffscreenHunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: func() string {
														if strings.HasPrefix(hunk.Header, "@@") {
															return hunk.Header
														}
														return "@@ " + hunk.Header
													}()})
													for _, indices := range func() [][2]int {
														rows := make([][2]int, 0, len(hunk.Lines))
														for index := 0; index < len(hunk.Lines); {
															if !v.split || hunk.Lines[index].Kind == patchContext {
																rows = append(rows, [2]int{index, index})
																index++
																continue
															}
															if hunk.Lines[index].Kind == patchAddition {
																rows = append(rows, [2]int{-1, index})
																index++
																continue
															}
															addedStart := func() int {
																var start int = index
																for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchDeletion {
																	start++
																}
																return start
															}()
															end := func() int {
																var start int = addedStart
																for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchAddition {
																	start++
																}
																return start
															}()
															rows = append(rows, func() [][2]int {
																count := max(addedStart-index, end-addedStart)
																rows := make([][2]int, 0, count)
																for offset := 0; offset < count; offset++ {
																	inlineLocalPair2 := [2]int{-1, -1}
																	if index+offset < addedStart {
																		inlineLocalPair2[0] = index + offset
																	}
																	if addedStart+offset < end {
																		inlineLocalPair2[1] = addedStart + offset
																	}
																	rows = append(rows, inlineLocalPair2)
																}
																return rows
															}()...)
															index = end
														}
														return rows
													}() {
														row := diffscreenEntry{kind: diffscreenLineRow, file: fileIndex, hunk: hunkIndex, leftLine: indices[0], rightLine: indices[1]}
														func() {
															var row *diffscreenEntry = &row
															if !v.split {
																row.text = func() string {
																	var line patchLine = hunk.Lines[row.rightLine]
																	if line.Kind == patchDeletion {
																		if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
																			return line.Text
																		}
																		return highlighted.Old[int(line.OldNumber)-1]
																	}
																	if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
																		return line.Text
																	}
																	return highlighted.New[int(line.NewNumber)-1]
																}()
																return
															}
															if row.leftLine >= 0 {
																row.left = func() string {
																	var line patchLine = hunk.Lines[row.leftLine]
																	if line.Kind == patchDeletion {
																		if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
																			return line.Text
																		}
																		return highlighted.Old[int(line.OldNumber)-1]
																	}
																	if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
																		return line.Text
																	}
																	return highlighted.New[int(line.NewNumber)-1]
																}()
															}
															if row.rightLine >= 0 {
																row.right = func() string {
																	var line patchLine = hunk.Lines[row.rightLine]
																	if line.Kind == patchDeletion {
																		if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
																			return line.Text
																		}
																		return highlighted.Old[int(line.OldNumber)-1]
																	}
																	if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
																		return line.Text
																	}
																	return highlighted.New[int(line.NewNumber)-1]
																}()
															}
														}()
														v.rows = append(v.rows, row)
													}
												}
											}
											return v
										}
									}()
									v.viewport = func(v *diffscreenDiffView, width, height int) diffscreenDiffViewport {
										var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
										viewport.Width, viewport.Height = max(1, width), max(1, height)
										{
											var viewport diffscreenDiffViewport = viewport
											viewport.top = max(0, min(viewport.top, func() int {
												end := max(0, len(v.rows)-viewport.Height)
												if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
													end++
												}
												return end
											}()))
											viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
												contentWidth := max(1, viewport.Width-14)
												extra := 0
												if v.split {
													contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
												}
												longest := 0
												for _, current := range v.rows {
													if current.kind != diffscreenLineRow {
														continue
													}
													longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
												}
												return max(0, longest-contentWidth)
											}()))
											return viewport
										}
									}(v.view, v.width, (func(height int) int { return max(1, height-3) })(v.height))
									v.cursor, _ = func() (diffscreenDiffCursor, bool) {
										var v *diffscreenDiffView = v.view
										if cursor, ok := func() (diffscreenDiffCursor, bool) {
											step := int(diffscreenPrivateForward)
											for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
												if cursor, ok := func() (diffscreenDiffCursor, bool) {
													cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
													return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
														var current diffscreenEntry = v.rows[cursor.row]
														if current.kind != diffscreenLineRow {
															return -1
														}
														if !v.split || cursor.pane == diffscreenRight {
															return current.rightLine
														}
														return current.leftLine
													}() >= 0
												}(); ok {
													return cursor, true
												}
											}
											return diffscreenDiffCursor{}, false
										}(); ok {
											return cursor, true
										}
										step := int(diffscreenPrivateForward)
										for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
											if cursor, ok := func() (diffscreenDiffCursor, bool) {
												cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
												return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
													var current diffscreenEntry = v.rows[cursor.row]
													if current.kind != diffscreenLineRow {
														return -1
													}
													if !v.split || cursor.pane == diffscreenRight {
														return current.rightLine
													}
													return current.leftLine
												}() >= 0
											}(); ok {
												return cursor, true
											}
										}
										return diffscreenDiffCursor{}, false
									}()
									return v
								})(currentPatch, m.diffOptions)
								m.diffView.Resize(m.width, m.height)
								m.commentView.Resize(m.width, m.height)
								return m
							}()
							m.delete = func(comment commentsComment, current patchPatch) error {
								return store.Delete(current.Root, comment.ID)
							}
							m.load = func() ([]commentsComment, error) { return store.List(currentPatch.Root) }
							return m, nil
						}()
						if err != nil {
							return err
						}
						m.ctx = ctx
						if refresh != nil {
							m.refresh = func(kind patchKind) (patchPatch, error) { return refresh(ctx, kind) }
						}
						return launch(
							m,
							tea.WithContext(ctx),
							tea.WithWindowSize(dimensions.Width, dimensions.Height),
						)
					})(
						ctx,
						currentPatch,
						inlineLocalCommentStore1,
						refresh,
						(func() appSize {
							if width, height, err := term.GetSize(os.Stdin.Fd()); err == nil {
								return appSize{Width: width, Height: height}
							}
							if width, height, err := term.GetSize(os.Stdout.Fd()); err == nil {
								return appSize{Width: width, Height: height}
							}
							return appDefaultSize
						}),
						func(
							m appModel, options ...tea.ProgramOption,
						) error {
							_, err := tea.NewProgram(m, options...).Run()
							return err
						},
					)
				})(ctx, currentPatch, commentStore, (func(ctx context.Context, kind patchKind) (patchPatch, error) {
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
				}))
			})(ctx)
		}
		if len(args) > 1 {
			return errors.New("usage: review-my-slop [code|comments]")
		}
		switch args[0] {
		case "code":
			return (func(ctx context.Context) error {
				commentStore, err := (func() (commentsStore, error) {
					path, err := (func() (string, error) {
						data, err := (func() (string, error) {
							var fallback string = filepath.Join(".local", "share")
							if root := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(root) {
								return filepath.Join(root, commentsAppName), nil
							}
							home, err := os.UserHomeDir()
							if err != nil {
								return "", fmt.Errorf("resolve user home directory: %w", err)
							}
							return filepath.Join(home, fallback, commentsAppName), nil
						})()
						if err != nil {
							return "", err
						}
						return filepath.Join(data, "comments.db"), nil
					})()
					if err != nil {
						return commentsStore{}, err
					}
					return commentsStore{Path: path}, nil
				})()
				if err != nil {
					return err
				}
				currentPatch, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
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
				if err != nil {
					return err
				}
				return (func(
					ctx context.Context,
					currentPatch patchPatch,
					inlineLocalCommentStore1 commentsStore,
					refresh func(context.Context, patchKind) (patchPatch, error),
				) error {
					return (func(
						ctx context.Context,
						currentPatch patchPatch,
						store appCommentStore, refresh func(context.Context, patchKind) (patchPatch, error),
						terminalSize func() appSize, launch func(appModel, ...tea.ProgramOption) error,
					) error {
						pending, err := store.List(currentPatch.Root)
						if err != nil {
							return err
						}
						dimensions := terminalSize()
						m, err := func() (appModel, error) {
							preferences, err := (func() (settingsPreferences, error) {
								path, err := (func() (string, error) {
									config, err := os.UserConfigDir()
									if err != nil {
										return "", fmt.Errorf("resolve UI settings directory: %w", err)
									}
									return filepath.Join(config, "review-my-slop", "ui.json"), nil
								})()
								if err != nil {
									return settingsPreferences{}, err
								}
								return (func(path string) (settingsPreferences, error) {
									data, err := os.ReadFile(path)
									if errors.Is(err, os.ErrNotExist) {
										return settingsPreferences{}, nil
									}
									if err != nil {
										return settingsPreferences{}, fmt.Errorf("read UI settings: %w", err)
									}
									var settings settingsPreferences
									if err := json.Unmarshal(data, &settings); err != nil {
										return settingsPreferences{}, fmt.Errorf("decode UI settings: %w", err)
									}
									return settings, nil
								})(path)
							})()
							if err != nil {
								return appModel{}, err
							}
							m := func() appModel {
								var layout appInitialLayout = appInitialLayout{SideBySide: preferences.SideBySide, SaveSideBySide: func(enabled bool) error {
									return (func(preferences settingsPreferences) error {
										path, err := (func() (string, error) {
											config, err := os.UserConfigDir()
											if err != nil {
												return "", fmt.Errorf("resolve UI settings directory: %w", err)
											}
											return filepath.Join(config, "review-my-slop", "ui.json"), nil
										})()
										if err != nil {
											return err
										}
										return (func(path string, preferences settingsPreferences) error {
											if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
												return fmt.Errorf("create UI settings directory: %w", err)
											}
											data, err := json.Marshal(preferences)
											if err != nil {
												return fmt.Errorf("encode UI settings: %w", err)
											}
											data = append(data, '\n')
											temporary, err := os.CreateTemp(filepath.Dir(path), "ui-*.tmp")
											if err != nil {
												return fmt.Errorf("create UI settings file: %w", err)
											}
											temporaryPath := temporary.Name()
											defer os.Remove(temporaryPath)
											if err := temporary.Chmod(0o600); err != nil {
												_ = temporary.Close()
												return fmt.Errorf("secure UI settings file: %w", err)
											}
											if _, err := temporary.Write(data); err != nil {
												_ = temporary.Close()
												return fmt.Errorf("write UI settings: %w", err)
											}
											if err := temporary.Close(); err != nil {
												return fmt.Errorf("close UI settings: %w", err)
											}
											if err := os.Rename(temporaryPath, path); err != nil {
												return fmt.Errorf("replace UI settings: %w", err)
											}
											return nil
										})(path, preferences)
									})(settingsPreferences{SideBySide: enabled})
								}, size: dimensions}
								inlineLocalSize1 := layout.size
								if inlineLocalSize1.Width <= 0 || inlineLocalSize1.Height <= 0 {
									inlineLocalSize1 = appDefaultSize
								}
								m := appModel{ctx: context.Background(), currentPatch: currentPatch, comments: appCommentState{items: pending}, commentView: (func(items []commentsComment) *commentscreenView {
									v := &commentscreenView{width: 80, height: 30}
									v.Update(items)
									return v
								})(pending), edit: appCommentEdit{index: -1}, width: inlineLocalSize1.Width, height: inlineLocalSize1.Height, save: func(comment commentsComment, current patchPatch) (commentsComment, error) {
									comment.Repository = current.Root
									if comment.ID != "" {
										if err := store.Update(comment); err != nil {
											return commentsComment{}, err
										}
										return comment, nil
									}
									return store.Add(comment)
								}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: currentPatch.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
									m := keymapMatcher{sequences: make(map[string]keymapSequence, len(sequences))}
									for _, sequence := range sequences {
										sequence.Keys = append([]string(nil), sequence.Keys...)
										m.sequences[sequence.Prefix] = sequence
									}
									return m
								})(keymapSequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymapSequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymapSequence{Prefix: "[", Keys: []string{"f"}}, keymapSequence{Prefix: "]", Keys: []string{"f"}}, keymapSequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
								m.diffView = (func(p patchPatch, options diffscreenOptions) *diffscreenView {
									v := &diffscreenView{options: options, width: 80, height: 30}
									v.view = func() *diffscreenDiffView {
										{
											var options diffscreenOptions = diffscreenOptions{SideBySide: v.options.SideBySide && v.width >= 100, Dark: v.options.Dark}
											v := &diffscreenDiffView{patch: p, split: options.SideBySide, dark: options.Dark}
											for fileIndex, file := range p.Files {
												var file patchFile = file
												v.rows = append(v.rows, diffscreenEntry{kind: diffscreenFileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
												for _, metadata := range file.Metadata {
													v.rows = append(v.rows, diffscreenEntry{kind: diffscreenMetadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
												}
												highlighted := func() diffscreenPair {
													var file *patchFile = &file
													theme := diffscreenLightTheme
													if v.dark {
														theme = diffscreenDarkTheme
													}
													return diffscreenPair{Old: func() []string {
														var filename, source, theme string = file.OldPath, file.OldSource, theme
														if source == "" {
															return nil
														}
														var buf bytes.Buffer
														highlightErr := quick.Highlight(&buf, source, filename, "terminal16m", theme)
														if highlightErr != nil {
															return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
														}
														rendered, err := io.ReadAll(&buf)
														if err != nil {
															return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
														}
														return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
													}(), New: func() []string {
														var filename, source, theme string = file.NewPath, file.NewSource, theme
														if source == "" {
															return nil
														}
														var buf bytes.Buffer
														highlightErr := quick.Highlight(&buf, source, filename, "terminal16m", theme)
														if highlightErr != nil {
															return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
														}
														rendered, err := io.ReadAll(&buf)
														if err != nil {
															return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
														}
														return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
													}()}
												}()
												for hunkIndex, hunk := range file.Hunks {
													v.rows = append(v.rows, diffscreenEntry{kind: diffscreenHunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: func() string {
														if strings.HasPrefix(hunk.Header, "@@") {
															return hunk.Header
														}
														return "@@ " + hunk.Header
													}()})
													for _, indices := range func() [][2]int {
														rows := make([][2]int, 0, len(hunk.Lines))
														for index := 0; index < len(hunk.Lines); {
															if !v.split || hunk.Lines[index].Kind == patchContext {
																rows = append(rows, [2]int{index, index})
																index++
																continue
															}
															if hunk.Lines[index].Kind == patchAddition {
																rows = append(rows, [2]int{-1, index})
																index++
																continue
															}
															addedStart := func() int {
																var start int = index
																for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchDeletion {
																	start++
																}
																return start
															}()
															end := func() int {
																var start int = addedStart
																for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchAddition {
																	start++
																}
																return start
															}()
															rows = append(rows, func() [][2]int {
																count := max(addedStart-index, end-addedStart)
																rows := make([][2]int, 0, count)
																for offset := 0; offset < count; offset++ {
																	inlineLocalPair2 := [2]int{-1, -1}
																	if index+offset < addedStart {
																		inlineLocalPair2[0] = index + offset
																	}
																	if addedStart+offset < end {
																		inlineLocalPair2[1] = addedStart + offset
																	}
																	rows = append(rows, inlineLocalPair2)
																}
																return rows
															}()...)
															index = end
														}
														return rows
													}() {
														row := diffscreenEntry{kind: diffscreenLineRow, file: fileIndex, hunk: hunkIndex, leftLine: indices[0], rightLine: indices[1]}
														func() {
															var row *diffscreenEntry = &row
															if !v.split {
																row.text = func() string {
																	var line patchLine = hunk.Lines[row.rightLine]
																	if line.Kind == patchDeletion {
																		if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
																			return line.Text
																		}
																		return highlighted.Old[int(line.OldNumber)-1]
																	}
																	if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
																		return line.Text
																	}
																	return highlighted.New[int(line.NewNumber)-1]
																}()
																return
															}
															if row.leftLine >= 0 {
																row.left = func() string {
																	var line patchLine = hunk.Lines[row.leftLine]
																	if line.Kind == patchDeletion {
																		if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
																			return line.Text
																		}
																		return highlighted.Old[int(line.OldNumber)-1]
																	}
																	if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
																		return line.Text
																	}
																	return highlighted.New[int(line.NewNumber)-1]
																}()
															}
															if row.rightLine >= 0 {
																row.right = func() string {
																	var line patchLine = hunk.Lines[row.rightLine]
																	if line.Kind == patchDeletion {
																		if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
																			return line.Text
																		}
																		return highlighted.Old[int(line.OldNumber)-1]
																	}
																	if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
																		return line.Text
																	}
																	return highlighted.New[int(line.NewNumber)-1]
																}()
															}
														}()
														v.rows = append(v.rows, row)
													}
												}
											}
											return v
										}
									}()
									v.viewport = func(v *diffscreenDiffView, width, height int) diffscreenDiffViewport {
										var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
										viewport.Width, viewport.Height = max(1, width), max(1, height)
										{
											var viewport diffscreenDiffViewport = viewport
											viewport.top = max(0, min(viewport.top, func() int {
												end := max(0, len(v.rows)-viewport.Height)
												if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
													end++
												}
												return end
											}()))
											viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
												contentWidth := max(1, viewport.Width-14)
												extra := 0
												if v.split {
													contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
												}
												longest := 0
												for _, current := range v.rows {
													if current.kind != diffscreenLineRow {
														continue
													}
													longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
												}
												return max(0, longest-contentWidth)
											}()))
											return viewport
										}
									}(v.view, v.width, (func(height int) int { return max(1, height-3) })(v.height))
									v.cursor, _ = func() (diffscreenDiffCursor, bool) {
										var v *diffscreenDiffView = v.view
										if cursor, ok := func() (diffscreenDiffCursor, bool) {
											step := int(diffscreenPrivateForward)
											for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
												if cursor, ok := func() (diffscreenDiffCursor, bool) {
													cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
													return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
														var current diffscreenEntry = v.rows[cursor.row]
														if current.kind != diffscreenLineRow {
															return -1
														}
														if !v.split || cursor.pane == diffscreenRight {
															return current.rightLine
														}
														return current.leftLine
													}() >= 0
												}(); ok {
													return cursor, true
												}
											}
											return diffscreenDiffCursor{}, false
										}(); ok {
											return cursor, true
										}
										step := int(diffscreenPrivateForward)
										for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
											if cursor, ok := func() (diffscreenDiffCursor, bool) {
												cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
												return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
													var current diffscreenEntry = v.rows[cursor.row]
													if current.kind != diffscreenLineRow {
														return -1
													}
													if !v.split || cursor.pane == diffscreenRight {
														return current.rightLine
													}
													return current.leftLine
												}() >= 0
											}(); ok {
												return cursor, true
											}
										}
										return diffscreenDiffCursor{}, false
									}()
									return v
								})(currentPatch, m.diffOptions)
								m.diffView.Resize(m.width, m.height)
								m.commentView.Resize(m.width, m.height)
								return m
							}()
							m.delete = func(comment commentsComment, current patchPatch) error {
								return store.Delete(current.Root, comment.ID)
							}
							m.load = func() ([]commentsComment, error) { return store.List(currentPatch.Root) }
							return m, nil
						}()
						if err != nil {
							return err
						}
						m.ctx = ctx
						if refresh != nil {
							m.refresh = func(kind patchKind) (patchPatch, error) { return refresh(ctx, kind) }
						}
						return launch(
							m,
							tea.WithContext(ctx),
							tea.WithWindowSize(dimensions.Width, dimensions.Height),
						)
					})(
						ctx,
						currentPatch,
						inlineLocalCommentStore1,
						refresh,
						(func() appSize {
							if width, height, err := term.GetSize(os.Stdin.Fd()); err == nil {
								return appSize{Width: width, Height: height}
							}
							if width, height, err := term.GetSize(os.Stdout.Fd()); err == nil {
								return appSize{Width: width, Height: height}
							}
							return appDefaultSize
						}),
						func(
							m appModel, options ...tea.ProgramOption,
						) error {
							_, err := tea.NewProgram(m, options...).Run()
							return err
						},
					)
				})(ctx, currentPatch, commentStore, (func(ctx context.Context, kind patchKind) (patchPatch, error) {
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
				}))
			})(ctx)
		case "comments":
			return (func(ctx context.Context, output io.Writer) error {
				commentStore, err := (func() (commentsStore, error) {
					path, err := (func() (string, error) {
						data, err := (func() (string, error) {
							var fallback string = filepath.Join(".local", "share")
							if root := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(root) {
								return filepath.Join(root, commentsAppName), nil
							}
							home, err := os.UserHomeDir()
							if err != nil {
								return "", fmt.Errorf("resolve user home directory: %w", err)
							}
							return filepath.Join(home, fallback, commentsAppName), nil
						})()
						if err != nil {
							return "", err
						}
						return filepath.Join(data, "comments.db"), nil
					})()
					if err != nil {
						return commentsStore{}, err
					}
					return commentsStore{Path: path}, nil
				})()
				if err != nil {
					return err
				}
				currentPatch, err := (func(ctx context.Context, kind patchKind) (patchPatch, error) {
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
				if err != nil {
					return err
				}
				return commentStore.WritePending(output, currentPatch.Root)
			})(ctx, output)
		default:
			return fmt.Errorf(
				"unknown subcommand %q; usage: review-my-slop [code|comments]",
				args[0],
			)
		}
	})(context.Background(), []string{"unknown"}, &bytes.Buffer{})
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, err != nil, true, "error = %v", err)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		strings.Contains(err.Error(), `unknown subcommand "unknown"`), true,
		"error = %v", err,
	)
}

type mainFailingWriter struct{}

func (mainFailingWriter) Write([]byte) (int, error) {
	return 0, os.ErrClosed
}
