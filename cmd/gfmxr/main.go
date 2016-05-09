package main

import (
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/urfave/gfmxr"
)

func main() {
	app := cli.NewApp()
	app.Name = "gfmxr"
	app.Authors = []cli.Author{
		cli.Author{
			Name:  "Dan Buch",
			Email: "daniel.buch@gmail.com",
		},
	}
	app.Version = gfmxr.VersionString
	app.Flags = []cli.Flag{
		cli.StringSliceFlag{
			Name:   "sources, s",
			Usage:  "markdown source(s) to search for runnable examples",
			EnvVar: "GFMXR_SOURCES,SOURCES",
		},
		cli.IntFlag{
			Name:   "count, c",
			Usage:  "expected count of runnable examples (for verification)",
			EnvVar: "GFMXR_COUNT,COUNT",
		},
		cli.BoolFlag{
			Name:   "debug, D",
			Usage:  "show debug output",
			EnvVar: "GFMXR_DEBUG,DEBUG",
		},
	}

	app.Action = func(ctx *cli.Context) {
		log := logrus.New()
		if ctx.Bool("debug") {
			log.Level = logrus.DebugLevel
		}

		sources := ctx.StringSlice("sources")
		count := ctx.Int("count")

		if len(sources) < 1 {
			sources = append(sources, "README.md")
		}

		errs := gfmxr.NewRunner(sources, count, log).Run()
		for _, err := range errs {
			log.Error(err)
		}

		if len(errs) > 0 {
			os.Exit(1)
		}

		os.Exit(0)
	}

	app.Run(os.Args)
}
