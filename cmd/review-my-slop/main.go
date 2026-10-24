// Command review-my-slop provides an interactive diff review and feedback tool.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/eskelinenantti/review-my-slop/internal/app"
	"github.com/eskelinenantti/review-my-slop/internal/appinfo"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

const (
	errorExitCode = 1
	usage         = "usage: " + appinfo.Name + " [code|comments]"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, appinfo.Name+":", err)
		os.Exit(errorExitCode)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	if len(args) == 0 {
		return runCode(ctx)
	}
	if len(args) > 1 {
		return errors.New(usage)
	}
	switch args[0] {
	case "code":
		return runCode(ctx)
	case "comments":
		return runComments(ctx, output)
	default:
		return fmt.Errorf("unknown subcommand %q; %s", args[0], usage)
	}
}

func runCode(ctx context.Context) error {
	commentStore, err := comments.OpenDefault()
	if err != nil {
		return err
	}
	currentPatch, err := patch.Get(ctx, patch.Unstaged)
	if err != nil {
		return err
	}
	return app.Run(ctx, currentPatch, commentStore, patch.Get)
}

func runComments(ctx context.Context, output io.Writer) error {
	commentStore, err := comments.OpenDefault()
	if err != nil {
		return err
	}
	root, err := patch.RepositoryRoot(ctx)
	if err != nil {
		return err
	}
	return commentStore.WritePending(output, root)
}
