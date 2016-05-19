package main

import (
	"os"

	"github.com/urfave/gfmxr"
)

func main() {
	gfmxr.NewCLI().Run(os.Args)
}
