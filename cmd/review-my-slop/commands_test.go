package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestRunRejectsUnknownSubcommand(t *testing.T) {
	err := run(context.Background(), []string{"unknown"}, &bytes.Buffer{})
	testutil.RequireEqual(t, err != nil, true, fmt.Sprintf("error = %v", err))
	testutil.RequireEqual(t,
		strings.Contains(err.Error(), `unknown subcommand "unknown"`), true,
		fmt.Sprintf("error = %v", err),
	)
}
