package gfmxr

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
)

type Runner struct {
	Sources   []string
	Count     int
	Frobs     map[string]Frob
	Languages *Languages

	log *logrus.Logger
}

func NewRunner(sources []string, count int, languagesYml string, autoPull bool, log *logrus.Logger) (*Runner, error) {
	var langs *Languages

	if languagesYml == "" {
		languagesYml = DefaultLanguagesYml
	}

	if _, err := os.Stat(languagesYml); err != nil && autoPull {
		log.WithFields(logrus.Fields{
			"url":  DefaultLanguagesYmlURL,
			"dest": languagesYml,
		}).Info("downloading")

		err = PullLanguagesYml(DefaultLanguagesYmlURL, languagesYml)
		if err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(languagesYml); err == nil {
		log.WithFields(logrus.Fields{
			"languages": languagesYml,
		}).Info("loading")

		langs, err = LoadLanguages(languagesYml)
		if err != nil {
			return nil, err
		}
	}

	return &Runner{
		Sources:   sources,
		Count:     count,
		Frobs:     DefaultFrobs,
		Languages: langs,

		log: log,
	}, nil
}

func (r *Runner) Run() []error {
	if len(r.Sources) < 1 {
		r.log.Warn("no sources given")
		return nil
	}

	res := []*runResult{}

	sourcesStart := time.Now()

	for i, sourceFile := range r.Sources {
		sourceBytes, err := ioutil.ReadFile(sourceFile)
		if err != nil {
			res = append(res, &runResult{Retcode: -1, Error: err})
			continue
		}

		res = append(res, r.checkSource(i, sourceFile, string(sourceBytes))...)
	}

	if r.Count > 0 && len(res) != r.Count {
		r.log.WithFields(logrus.Fields{
			"expected": r.Count,
			"actual":   len(res),
		}).Error("mismatched example count")

		return []error{fmt.Errorf("example count %d != expected %d", len(res), r.Count)}
	}

	if len(res) == 0 {
		return []error{}
	}

	errs := []error{}

	for _, result := range res {
		if result == nil {
			continue
		}

		if result.Stdout != "" || result.Stderr != "" {
			r.log.WithFields(logrus.Fields{
				"source": result.Runnable.SourceFile,
				"stdout": result.Stdout,
				"stderr": result.Stderr,
			}).Debug("captured output")
		}

		if result.Error != nil {
			errs = append(errs, result.Error)
		}
	}

	r.log.WithFields(logrus.Fields{
		"source_count":  len(r.Sources),
		"example_count": len(res),
		"error_count":   len(errs),
		"time":          time.Now().Sub(sourcesStart),
	}).Info("done")

	return errs
}

func (r *Runner) checkSource(i int, sourceName, source string) []*runResult {
	res := []*runResult{}
	sourceStart := time.Now()
	runnables := r.findRunnables(i, sourceName, source)

	for j, runnable := range runnables {
		r.log.WithFields(logrus.Fields{
			"i":      fmt.Sprintf("%d/%d", j+1, len(runnables)),
			"source": sourceName,
			"line":   runnable.LineOffset,
			"lang":   runnable.Lang,
		}).Info("start")

		start := time.Now()
		res = append(res, runnable.Run(j))
		end := time.Now().Sub(start)

		r.log.WithFields(logrus.Fields{
			"i":      fmt.Sprintf("%d/%d", j+1, len(runnables)),
			"source": sourceName,
			"line":   runnable.LineOffset,
			"lang":   runnable.Lang,
			"time":   end,
		}).Info("finish")
	}

	r.log.WithFields(logrus.Fields{
		"source": sourceName,
		"time":   time.Now().Sub(sourceStart),
	}).Info("checked")

	return res
}

func (r *Runner) findRunnables(i int, sourceName, source string) []*Runnable {
	finder := newRunnableFinder(sourceName, source, r.log)
	runnables := finder.Find()

	filteredRunnables := []*Runnable{}
	for _, runnable := range runnables {
		exe, ok := r.Frobs[runnable.Lang]
		if !ok && r.Languages != nil {
			lang := r.Languages.Lookup(runnable.Lang)

			if lang == nil {
				r.log.WithFields(logrus.Fields{
					"source": runnable.SourceFile,
					"lineno": runnable.LineOffset,
					"lang":   runnable.Lang,
				}).Debug("unknown language, skipping")
				continue
			}

			runnable.Lang = lang.Name
			exe, ok = r.Frobs[runnable.Lang]
		}

		if !ok {
			r.log.WithFields(logrus.Fields{
				"source": runnable.SourceFile,
				"lineno": runnable.LineOffset,
				"lang":   runnable.Lang,
			}).Debug("no executor available for lang")
			continue
		}

		runnable.Frob = exe

		if err := exe.CanExecute(runnable); err != nil {
			r.log.WithFields(logrus.Fields{
				"source": runnable.SourceFile,
				"lineno": runnable.LineOffset,
				"reason": err,
			}).Debug("skipping runnable due to filter func")
			continue
		}

		filteredRunnables = append(filteredRunnables, runnable)
	}

	r.log.WithField("runnable_count", len(filteredRunnables)).Debug("returning runnables")
	return filteredRunnables
}
