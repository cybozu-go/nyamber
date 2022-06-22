package main

import (
	"os"

	"github.com/cybozu-go/nyamber/cmd/entrypoint/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
