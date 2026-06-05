package main

import (
	"os"

	"hydra-gitops.org/hydra/hydra-go/cli/cmd"
	"hydra-gitops.org/hydra/hydra-go/cli/exitcode"
)

func main() {
	err := cmd.Execute()
	if err != nil {
		if code, ok := exitcode.As(err); ok {
			os.Exit(code)
		}
		os.Exit(1)
	}
}
