package gfmrun

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Sirupsen/logrus"
	"gopkg.in/urfave/cli.v2"
)

func NewCLI() *cli.App {
	return &cli.App{
		Name:    "gfmrun",
		Usage:   "github-flavored markdown example runner",
		Version: VersionString,
		Authors: []*cli.Author{
			{
				Name:  "Dan Buch",
				Email: "dan@meatballhat.com",
			},
		},
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:    "sources",
				Aliases: []string{"s"},
				Usage:   "markdown source(s) to search for runnable examples",
				Value:   cli.NewStringSlice("README.md"),
				EnvVars: []string{"GFMRUN_SOURCES", "SOURCES"},
			},
			&cli.IntFlag{
				Name:    "count",
				Aliases: []string{"c"},
				Usage:   "expected count of runnable examples (for verification)",
				EnvVars: []string{"GFMRUN_COUNT", "COUNT"},
			},
			&cli.StringFlag{
				Name:    "languages",
				Aliases: []string{"L"},
				Usage:   "location of languages.yml file from linguist",
				Value:   DefaultLanguagesYml,
				EnvVars: []string{"GFMRUN_LANGUAGES", "LANGUAGES"},
			},
			&cli.BoolFlag{
				Name:    "no-auto-pull",
				Aliases: []string{"N"},
				Value:   true,
				Usage:   "disable automatic pull of languages.yml when missing",
				EnvVars: []string{"GFMRUN_NO_AUTO_PULL", "NO_AUTO_PULL"},
			},
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"D"},
				Usage:   "show debug output",
				EnvVars: []string{"GFMRUN_DEBUG", "DEBUG"},
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "pull-languages",
				Usage: "explicitly download the latest languages.yml from the linguist source to $GFMRUN_LANGUAGES (automatic unless \"--no-auto-pull\")",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "languages-url",
						Aliases: []string{"u"},
						Usage:   "source URL of languages.yml file from linguist",
						Value:   DefaultLanguagesYmlURL,
						EnvVars: []string{"GFMRUN_LANGUAGES_URL", "LANGUAGES_URL"},
					},
				},
				Action: cliPullLanguages,
			},
			{
				Name:   "dump-languages",
				Usage:  "dump the parsed languages data structure as JSON",
				Hidden: true,
				Action: cliDumpLanguages,
			},
			{
				Name:   "list-frobs",
				Usage:  "list the known frobs and handled frob aliases",
				Hidden: true,
				Action: cliListFrobs,
			},
			{
				Name:  "extract",
				Usage: "extract examples to files",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "output-dir",
						Aliases: []string{"o"},
						Usage:   "output directory for extracted examples",
						Value:   os.TempDir(),
						EnvVars: []string{"GFMRUN_OUTPUT_DIR", "OUTPUT_DIR"},
					},
				},
				Action: cliExtract,
			},
		},
		Action: cliRunExamples,
	}
}

func RunExamples(sources []string, expectedCount int, languagesFile string, autoPull bool, log *logrus.Logger) error {
	if sources == nil {
		sources = []string{}
	}

	if log == nil {
		log = logrus.New()
	}

	runner, err := NewRunner(sources, expectedCount, languagesFile, autoPull, log)
	if err != nil {
		return err
	}

	errs := runner.Run()

	if len(errs) > 0 {
		msg := make([]string, len(errs))
		for i, err := range errs {
			msg[i] = err.Error()
		}
		return errors.New(strings.Join(msg, "\n"))
	}

	return nil
}

func ExtractExamples(sources []string, outDir, languagesFile string, autoPull bool, log *logrus.Logger) error {
	if sources == nil {
		sources = []string{}
	}

	if log == nil {
		log = logrus.New()
	}

	outDirFd, err := os.Stat(outDir)
	if err == nil && !outDirFd.IsDir() {
		return fmt.Errorf("output path %q must be a directory or nonexistent", outDir)
	}

	if err != nil {
		err = os.MkdirAll(outDir, os.FileMode(0750))
	}

	if err != nil {
		return err
	}

	runner, err := NewRunner(sources, 0, languagesFile, autoPull, log)
	if err != nil {
		return err
	}

	runner.noExec = true
	runner.extractDir = outDir
	errs := runner.Run()

	if len(errs) > 0 {
		msg := make([]string, len(errs))
		for i, err := range errs {
			msg[i] = err.Error()
		}
		return errors.New(strings.Join(msg, "\n"))
	}

	return nil
}

func cliRunExamples(ctx *cli.Context) error {
	log := logrus.New()
	if ctx.Bool("debug") {
		log.Level = logrus.DebugLevel
	}

	err := RunExamples(ctx.StringSlice("sources"), ctx.Int("count"),
		ctx.String("languages"), ctx.Bool("no-auto-pull"), log)

	if err != nil {
		log.Error(err)
		return cli.Exit("", 2)
	}

	return nil
}

func cliListFrobs(ctx *cli.Context) error {
	langs, err := LoadLanguages(ctx.String("languages"))
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
	log := logrus.New()
	if ctx.Bool("debug") {
		log.Level = logrus.DebugLevel
	}

	langs, err := LoadLanguages(ctx.String("languages"))
	if err != nil {
		log.Error(err)
		return cli.Exit("failed to load languages", 4)
	}

	jsonBytes, err := json.MarshalIndent(langs.Map, "", "  ")
	if err != nil {
		log.Error(err)
		return cli.Exit("failed to marshal to json", 4)
	}

	fmt.Printf(string(jsonBytes) + "\n")
	return nil
}

func cliPullLanguages(ctx *cli.Context) error {
	err := PullLanguagesYml(ctx.String("languages-url"), ctx.String("languages"))
	if err != nil {
		return cli.Exit(err.Error(), 2)
	}
	return nil
}

func cliExtract(ctx *cli.Context) error {
	log := logrus.New()
	if ctx.Bool("debug") {
		log.Level = logrus.DebugLevel
	}

	err := ExtractExamples(ctx.StringSlice("sources"), ctx.String("output-dir"),
		ctx.String("languages"), ctx.Bool("no-auto-pull"), log)

	if err != nil {
		log.Error(err)
		return cli.Exit("", 2)
	}

	return nil
}
