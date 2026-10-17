package patch

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sourcegraph/go-diff/diff"
)

const maxFileBytes = 2 << 20

type runner interface {
	Run(ctx context.Context, dir string, args ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(
	ctx context.Context,
	dir string,
	args ...string,
) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_PAGER=cat",
		"GIT_EXTERNAL_DIFF=",
		"GIT_CONFIG_NOSYSTEM=1",
	)
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), ctx.Err())
	}
	if err == nil {
		return out, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return nil, fmt.Errorf(
			"git %s: %s",
			strings.Join(args, " "),
			strings.TrimSpace(string(exitErr.Stderr)),
		)
	}
	return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}

// Get retrieves a complete snapshot from the current working directory.
// Unknown kinds and unavailable branch comparisons return a zero Patch and an
// error.
func Get(ctx context.Context, kind Kind) (Patch, error) {
	directory, err := os.Getwd()
	if err != nil {
		return Patch{}, fmt.Errorf("resolve working directory: %w", err)
	}
	{
		if kind != Unstaged && kind != Branch {
			return Patch{}, fmt.Errorf("unknown patch kind %d", kind)
		}
		if err := ctx.Err(); err != nil {
			return Patch{}, err
		}
		root, err := func() (string, error) {
			rootBytes, err := runner(execRunner{}).Run(ctx, directory, "rev-parse", "--show-toplevel")
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
			return Patch{}, err
		}
		branch, err := func() (string, error) {
			out, err := runner(execRunner{}).Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			if err == nil {
				return strings.TrimSpace(string(out)), nil
			}
			for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
				_, err := runner(execRunner{}).Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
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
			return Patch{}, fmt.Errorf("discover default branch: %w", err)
		}
		revisions, readOld, err := func() ([]string, sourceReader, error) {
			if kind != Branch {
				return nil, func(ctx context.Context, run runner, root, path string) string {
					if path == "" || path == "/dev/null" {
						return ""
					}
					out, err := run.Run(ctx, root, "show", ":"+path)
					if err != nil || len(out) > maxFileBytes || bytes.IndexByte(out, 0) >= 0 {
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
			baseBytes, err := runner(execRunner{}).Run(ctx, root, "merge-base", branch, "HEAD")
			if err != nil {
				return nil, nil, fmt.Errorf("find branch point with %s: %w", branch, err)
			}
			base := strings.TrimSpace(string(baseBytes))
			readOld := func(ctx context.Context, run runner, root, path string) string {
				if path == "" || path == "/dev/null" {
					return ""
				}
				out, err := run.Run(ctx, root, "show", base+":"+path)
				if err != nil || len(out) > maxFileBytes || bytes.IndexByte(out, 0) >= 0 {
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
			return Patch{}, err
		}
		raw, err := func() ([]byte, error) {
			args := []string{"-c", "core.quotepath=false", "-c", "diff.external=", "--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames", "--src-prefix=a/", "--dst-prefix=b/", "--unified=3"}
			args = append(args, revisions...)
			args = append(args, "--")
			return runner(execRunner{}).Run(ctx, root, args...)
		}()
		if err != nil {
			return Patch{}, err
		}
		{
			files, err := func() ([]File, error) {
				if len(bytes.TrimSpace(raw)) == 0 {
					return nil, nil
				}
				parsed, err := diff.ParseMultiFileDiff(raw)
				if err != nil {
					return nil, fmt.Errorf("parse git diff: %w", err)
				}
				files := make([]File, 0, len(parsed))
				for _, fd := range parsed {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					file, err := func() (File, error) {
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
						file := File{OldPath: oldPath, NewPath: newPath, DisplayPath: func() string {
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
						file.OldSource = readOld(ctx, execRunner{}, root, oldPath)
						file.NewSource = func() string {
							if newPath == "" || newPath == "/dev/null" {
								return ""
							}
							full := filepath.Join(root, filepath.FromSlash(newPath))
							info, err := os.Lstat(full)
							if err != nil || !info.Mode().IsRegular() || info.Size() > maxFileBytes {
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
							lines, err := func() ([]Line, error) {
								var (
									oldLine, newLine int32  = h.OrigStartLine, h.NewStartLine
									body             []byte = h.Body
								)
								rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
								if len(body) == 0 {
									return []Line{}, nil
								}
								lines := make([]Line, 0, len(rawLines))
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
										lines = append(lines, Line{Kind: Context, Text: text, OldNumber: LineNumber(oldLine), NewNumber: LineNumber(newLine)})
										oldLine++
										newLine++
									case '+':
										lines = append(lines, Line{Kind: Addition, Text: text, NewNumber: LineNumber(newLine)})
										newLine++
									case '-':
										lines = append(lines, Line{Kind: Deletion, Text: text, OldNumber: LineNumber(oldLine)})
										oldLine++
									case '\\':
									default:
										return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
									}
								}
								return lines, nil
							}()
							if err != nil {
								return File{}, fmt.Errorf("%s: %w", display, err)
							}
							file.Hunks = append(file.Hunks, Hunk{Header: func() string {
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
				return Patch{}, err
			}
			untracked, err := func() ([]File, error) {
				out, err := runner(execRunner{}).Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
				if err != nil {
					return nil, err
				}
				var files []File
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
					file, include, err := func() (File, bool, error) {
						full := filepath.Join(root, filepath.FromSlash(path))
						info, err := os.Lstat(full)
						if err != nil {
							return File{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
						}
						if info.Mode()&os.ModeSymlink != 0 {
							target, err := os.Readlink(full)
							if err != nil {
								return File{}, false, fmt.Errorf("read symlink %q: %w", path, err)
							}
							return func() File {
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
								lines := make([]Line, 0, len(sourceLines))
								for i, line := range sourceLines {
									lines = append(lines, Line{Kind: Addition, Text: line, NewNumber: LineNumber(i + 1)})
								}
								return File{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
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
								}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []Hunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
							}(), true, nil
						}
						if !info.Mode().IsRegular() {
							return File{}, false, nil
						}
						if info.Size() > maxFileBytes {
							return File{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
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
							return File{}, false, fmt.Errorf("read untracked %q: %w", path, err)
						}
						if bytes.IndexByte(content, 0) >= 0 {
							return File{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
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
						return func() File {
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
							lines := make([]Line, 0, len(sourceLines))
							for i, line := range sourceLines {
								lines = append(lines, Line{Kind: Addition, Text: line, NewNumber: LineNumber(i + 1)})
							}
							return File{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
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
							}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []Hunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
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
				return Patch{}, err
			}
			files = append(files, untracked...)
			slices.SortStableFunc(files, func(a, b File) int { return cmp.Compare(a.DisplayPath, b.DisplayPath) })
			if err := ctx.Err(); err != nil {
				return Patch{}, err
			}
			return Patch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
		}
	}
}

type sourceReader func(context.Context, runner, string, string) string
