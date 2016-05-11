package gfmxr

import (
	"encoding/json"
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
		cli.StringFlag{
			Name:   "languages, L",
			Usage:  "location of languages.yml file from linguist",
			Value:  DefaultLanguagesYml,
			EnvVar: "GFMXR_LANGUAGES,LANGUAGES",
		},
	}

	app.Commands = []cli.Command{
		cli.Command{
			Name:  "pull-languages",
			Usage: "download the latest languages.yml from the linguist source to $GFMXR_LANGUAGES",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:   "languages-url, u",
					Usage:  "source URL of languages.yml file from linguist",
					Value:  DefaultLanguagesYmlURL,
					EnvVar: "GFMXR_LANGUAGES_URL,LANGUAGES_URL",
				},
			},
			Action: cliPullLanguages,
		},
		cli.Command{
			Name:   "dump-languages",
			Usage:  "dump the parsed languages data structure as JSON",
			Hidden: true,
			Action: cliDumpLanguages,
		},
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

func RunExamples(sources []string, expectedCount int, languages string, log *logrus.Logger) error {
	if sources == nil {
		sources = []string{}
	}

	if log == nil {
		log = logrus.New()
	}

	if len(sources) < 1 {
		sources = append(sources, "README.md")
	}

	runner, err := NewRunner(sources, expectedCount, languages, log)
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

	sources := ctx.StringSlice("sources")
	count := ctx.Int("count")
	languages := ctx.String("languages")

	if len(sources) < 1 {
		sources = append(sources, "README.md")
	}

	runner, err := NewRunner(sources, count, languages, log)
	if err != nil {
		log.Error(err)
		return cli.NewExitError("", 2)
	}

	errs := runner.Run()
	for _, err := range errs {
		log.Error(err)
	}

	if len(errs) > 0 {
		return cli.NewExitError("", 1)
	}

	return nil
}

func cliListFrobs(ctx *cli.Context) error {
	langs, err := LoadLanguages(ctx.GlobalString("languages"))
	if err != nil {
		return err
	}

	known := map[string]bool{}

	for name, _ := range DefaultFrobs {
		for _, alias := range langs.Lookup(name).Aliases {
			known[alias] = true
		}
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

func cliDumpLanguages(ctx *cli.Context) error {
	langs, err := LoadLanguages(ctx.GlobalString("languages"))
	if err != nil {
		return cli.NewMultiError(cli.NewExitError("failed to load languages", 4), err)
	}

	jsonBytes, err := json.MarshalIndent(langs.Map, "", "  ")
	if err != nil {
		return cli.NewMultiError(cli.NewExitError("failed to marshal to json", 4), err)
	}

	fmt.Printf(string(jsonBytes) + "\n")
	return nil
}

func cliPullLanguages(ctx *cli.Context) error {
	return PullLanguagesYml(ctx.String("languages-url"), ctx.GlobalString("languages"))
}
