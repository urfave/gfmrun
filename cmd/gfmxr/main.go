package main

import (
	"fmt"
	"os"

	"github.com/urfave/gfmxr"
)

func main() {
	err := gfmxr.NewCLI().Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}
