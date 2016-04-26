package main

import (
	"fmt"
	"os"

	"github.com/meatballhat/gfmxr"
)

func main() {
	errs := gfmxr.NewRunner([]string{"./README.md"}, 0).Run()
	for _, err := range errs {
		fmt.Printf("ERROR: %#v\n", err)
	}

	if len(errs) > 0 {
		os.Exit(1)
	}

	os.Exit(0)
}
