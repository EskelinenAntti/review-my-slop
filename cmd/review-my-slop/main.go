// Command review-my-slop provides an interactive diff review and feedback tool.
package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/eskelinenantti/review-my-slop/internal/app"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"io"
	"os"
)

const errorExitCode = 1

func main() {
	if err := (func(ctx context.Context, args []string, output io.Writer) error {
		if len(args) == 0 {
			return (func(ctx context.Context) error {
				commentStore, err := comments.OpenDefault()
				if err != nil {
					return err
				}
				currentPatch, err := patch.Get(ctx, patch.Unstaged)
				if err != nil {
					return err
				}
				return app.Run(ctx, currentPatch, commentStore, patch.Get)
			})(ctx)
		}
		if len(args) > 1 {
			return errors.New("usage: review-my-slop [code|comments]")
		}
		switch args[0] {
		case "code":
			return (func(ctx context.Context) error {
				commentStore, err := comments.OpenDefault()
				if err != nil {
					return err
				}
				currentPatch, err := patch.Get(ctx, patch.Unstaged)
				if err != nil {
					return err
				}
				return app.Run(ctx, currentPatch, commentStore, patch.Get)
			})(ctx)
		case "comments":
			return (func(ctx context.Context, output io.Writer) error {
				commentStore, err := comments.OpenDefault()
				if err != nil {
					return err
				}
				currentPatch, err := patch.Get(ctx, patch.Unstaged)
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
	})(context.Background(), os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "review-my-slop:", err)
		os.Exit(errorExitCode)
	}
}
