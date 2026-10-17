// Command review-my-slop provides interactive patch review.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/eskelinenantti/review-my-slop/internal/app"
)

const errorExitCode = 1

func main() {
	err := app.Run(context.Background(), os.Args[1:], os.Stdout)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "review-my-slop:", err)
		os.Exit(errorExitCode)
	}
}
