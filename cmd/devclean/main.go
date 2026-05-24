package main

import (
	"fmt"
	"os"

	"github.com/maharabhossain1/devclean/cmd/devclean/commands"
)

func main() {
	if err := commands.Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
