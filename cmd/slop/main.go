package main

import (
	"fmt"
	"os"

	slop "github.com/anttieskelinen/review-my-slop"
)

func main() {
	if err := slop.Run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "slop:", err)
		os.Exit(1)
	}
}
