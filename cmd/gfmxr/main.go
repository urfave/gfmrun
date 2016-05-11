package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

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
			Email: "dan@meatballhat.com",
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
		cli.StringFlag{
			Name:   "languages, L",
			Usage:  "location of languages.yml file from linguist",
			Value:  gfmxr.DefaultLanguagesYml,
			EnvVar: "GFMXR_LANGUAGES,LANGUAGES",
		},
	}

	app.Commands = []cli.Command{
		cli.Command{
			Name:  "pull-languages",
			Usage: "explicitly download the latest languages.yml from the linguist source to $GFMXR_LANGUAGES (automatic otherwise)",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:   "languages-url, u",
					Usage:  "source URL of languages.yml file from linguist",
					Value:  gfmxr.DefaultLanguagesYmlURL,
					EnvVar: "GFMXR_LANGUAGES_URL,LANGUAGES_URL",
				},
			},
			Action: func(ctx *cli.Context) error {
				return gfmxr.PullLanguagesYml(ctx.String("languages-url"), ctx.GlobalString("languages"))
			},
		},
		cli.Command{
			Name:   "dump-languages",
			Usage:  "dump the parsed languages data structure as JSON",
			Hidden: true,
			Action: func(ctx *cli.Context) error {
				langs, err := gfmxr.LoadLanguages(ctx.GlobalString("languages"))
				if err != nil {
					return cli.NewMultiError(cli.NewExitError("failed to load languages", 4), err)
				}

				jsonBytes, err := json.MarshalIndent(langs.Map, "", "  ")
				if err != nil {
					return cli.NewMultiError(cli.NewExitError("failed to marshal to json", 4), err)
				}

				fmt.Printf(string(jsonBytes) + "\n")
				return nil
			},
		},
		cli.Command{
			Name:   "list-frobs",
			Usage:  "list the known frobs and handled frob aliases",
			Hidden: true,
			Action: func(ctx *cli.Context) error {
				langs, err := gfmxr.LoadLanguages(ctx.GlobalString("languages"))
				if err != nil {
					return err
				}

				known := map[string]bool{}

				for name, _ := range gfmxr.DefaultFrobs {
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
			},
		},
	}

	app.Action = func(ctx *cli.Context) error {
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

		runner, err := gfmxr.NewRunner(sources, count, languages, log)
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

	app.Run(os.Args)
}
