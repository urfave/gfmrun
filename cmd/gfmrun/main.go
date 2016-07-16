package main

import (
	"os"

	"github.com/urfave/gfmrun"
)

func main() {
	gfmrun.NewCLI().Run(os.Args)
}
