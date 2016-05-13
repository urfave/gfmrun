package gfmxr

import (
	"fmt"

	"github.com/codegangsta/cli"
)

var (
	VersionString = "0.1.0"
)

func init() {
	cli.VersionPrinter = printVersion
}

func printVersion(_ *cli.Context) {
	fmt.Printf("%s\n", VersionString)
}
