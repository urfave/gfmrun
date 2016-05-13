package gfmxr

import (
	"fmt"
	"sort"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

func NewCLI() *cli.App {
	app := cli.NewApp()
	app.Name = "gfmxr"
	app.Usage = "github-flavored markdown example runner"
	app.Authors = []cli.Author{
		cli.Author{
			Name:  "Dan Buch",
			Email: "dan@meatballhat.com",
		},
	}
	app.Version = VersionString
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

	app.Commands = []cli.Command{
		cli.Command{
			Name:   "list-frobs",
			Usage:  "list the known frobs and handled frob aliases",
			Hidden: true,
			Action: cliListFrobs,
		},
	}

	app.Action = cliRunExamples

	return app
}

func RunExamples(sources []string, expectedCount int, log *logrus.Logger) error {
	if sources == nil {
		sources = []string{}
	}

	if log == nil {
		log = logrus.New()
	}

	if len(sources) < 1 {
		sources = append(sources, "README.md")
	}

	runner, err := NewRunner(sources, expectedCount, log)
	if err != nil {
		return err
	}

	errs := runner.Run()

	if len(errs) > 0 {
		return cli.NewMultiError(errs...)
	}

	return nil
}

func cliRunExamples(ctx *cli.Context) error {
	log := logrus.New()
	if ctx.Bool("debug") {
		log.Level = logrus.DebugLevel
	}

	err := RunExamples(ctx.StringSlice("sources"), ctx.Int("count"), log)

	if err != nil {
		log.Error(err)
		return cli.NewMultiError(err, cli.NewExitError("", 2))
	}

	return nil
}

func cliListFrobs(ctx *cli.Context) error {
	known := map[string]bool{}

	for name, _ := range DefaultFrobs {
		known[name] = true
	}

	knownSlice := []string{}
	for lang := range known {
		knownSlice = append(knownSlice, lang)
	}

	sort.Strings(knownSlice)

	for _, lang := range knownSlice {
		fmt.Printf("%s\n", lang)
	}

	return nil
}
