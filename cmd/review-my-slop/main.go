package main

import (
	"context"
	"fmt"
	"os"

	"github.com/eskelinenantti/review-my-slop/internal/ui"
)

func main() {
	if err := ui.Run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "review-my-slop:", err)
		os.Exit(1)
	}
}
