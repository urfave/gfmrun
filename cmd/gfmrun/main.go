package main

import (
	"log"
	"os"

	"github.com/urfave/gfmrun"
)

func main() {
	err := gfmrun.NewCLI().Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
