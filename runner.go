package gfmxr

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/Sirupsen/logrus"
)

var (
	DefaultRunFilters = map[string]RunFilterFunc{
		"go": func(rn *Runnable) bool {
			return len(rn.Lines) > 0 && strings.TrimSpace(rn.Lines[0]) == "package main"
		},
		"python": func(_ *Runnable) bool { return true },
	}
	DefaultExecutors = map[string][]string{
		"go": []string{"go", "run", "$FILE"},
	}
	DefaultFileExtensions = map[string]string{
		"go":     "go",
		"python": "py",
	}
)

type Runner struct {
	Sources        []string
	Count          int
	Executors      map[string][]string
	FileExtensions map[string]string
	RunFilters     map[string]RunFilterFunc

	log *logrus.Logger
}

func NewRunner(sources []string, count int, log *logrus.Logger) *Runner {
	return &Runner{
		Sources:        sources,
		Count:          count,
		Executors:      DefaultExecutors,
		FileExtensions: DefaultFileExtensions,
		RunFilters:     DefaultRunFilters,

		log: log,
	}
}

func (r *Runner) Run() []error {
	if len(r.Sources) < 1 {
		r.log.Warn("no sources given")
		return nil
	}

	res := []*runResult{}

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

	return errs
}

func (r *Runner) checkSource(i int, sourceName, source string) []*runResult {
	res := []*runResult{}

	for j, runnable := range r.findRunnables(i, sourceName, source) {
		res = append(res, r.runRunnable(j, runnable))
	}

	r.log.WithFields(logrus.Fields{"source": sourceName}).Info("checked")

	return res
}

type mdState int

const (
	mdStateText mdState = iota
	mdStateCodeBlock
	mdStateRunnable
	mdStateComment
)

// custom markdown parser egad
// (because blackfriday doesn't give us line numbers (???))
func (r *Runner) findRunnables(i int, sourceName, source string) []*Runnable {
	runnables := []*Runnable{}
	cur := &Runnable{SourceFile: sourceName}
	state := mdStateText
	lastLine := ""

	for j, line := range strings.Split(source, "\n") {
		trimmedLine := strings.TrimSpace(line)
		r.log.WithFields(logrus.Fields{
			"source":      i,
			"source_name": sourceName,
			"lineno":      j,
			"line":        trimmedLine,
			"state":       state,
		}).Debug("scanning")
		lastLine = line

		if strings.HasPrefix(trimmedLine, "```") || strings.HasPrefix(trimmedLine, "~~~") {
			if state == mdStateCodeBlock && trimmedLine == cur.BlockStart {
				r.log.Debug("leaving non-runnable code block")
				state = mdStateText
				continue
			}

			if state == mdStateRunnable && trimmedLine == cur.BlockStart {
				runnables = append(runnables, cur)
				cur = &Runnable{SourceFile: sourceName}
				r.log.WithField("runnable_count", len(runnables)).Debug("leaving runnable code block")
				state = mdStateText
				continue
			}

			unGated := strings.Replace(strings.Replace(trimmedLine, "`", "", -1), "~", "", -1)
			if len(unGated) > 0 {
				r.log.WithField("lineno", j).Debug("starting new runnable")
				cur.Begin(j, trimmedLine)
				state = mdStateRunnable
				continue
			}

			r.log.WithField("lineno", j).Debug("starting new non-runnable code block")
			state = mdStateCodeBlock

		} else if strings.HasPrefix(trimmedLine, "<!--") {
			if state != mdStateText {
				state = mdStateComment
			}
		} else if strings.HasPrefix(trimmedLine, "-->") {
			if state == mdStateComment {
				state = mdStateText
			}
		} else {
			if state == mdStateRunnable {
				cur.Lines = append(cur.Lines, line)
			}
		}
	}

	if state == mdStateCodeBlock {
		// whatever, let's give it a shot
		cur.Lines = append(cur.Lines, lastLine)
		runnables = append(runnables, cur)
	}

	filteredRunnables := []*Runnable{}
	for _, runnable := range runnables {
		ff, ok := r.RunFilters[runnable.Lang]

		if !ok {
			r.log.WithFields(logrus.Fields{
				"source": runnable.SourceFile,
				"lineno": runnable.LineOffset,
				"lang":   runnable.Lang,
			}).Debug("no filter func available for lang")
			continue
		}

		if !ff(runnable) {
			r.log.WithFields(logrus.Fields{
				"source": runnable.SourceFile,
				"lineno": runnable.LineOffset,
			}).Debug("skipping runnable due to filter func")
			continue
		}

		filteredRunnables = append(filteredRunnables, runnable)
	}

	r.log.WithField("runnable_count", len(filteredRunnables)).Debug("returning runnables")
	return filteredRunnables
}

func (r *Runner) runRunnable(i int, rn *Runnable) *runResult {
	exe, ok := r.Executors[rn.Lang]
	if !ok {
		return &runResult{
			Runnable: rn,
			Retcode:  -1,
			Error:    fmt.Errorf("no executor available for lang %q", rn.Lang),
		}
	}

	ext, ok := r.FileExtensions[rn.Lang]
	if !ok {
		return &runResult{
			Runnable: rn,
			Retcode:  -1,
			Error:    fmt.Errorf("no known file extension for lang %q", rn.Lang),
		}
	}

	tmpFile, err := ioutil.TempFile("", "gfmxr")
	if err != nil {
		return &runResult{Runnable: rn, Retcode: -1, Error: err}
	}

	if _, err := tmpFile.Write([]byte(rn.String())); err != nil {
		return &runResult{Runnable: rn, Retcode: -1, Error: err}
	}

	if err := tmpFile.Close(); err != nil {
		return &runResult{Runnable: rn, Retcode: -1, Error: err}
	}

	tmpFileWithExt := fmt.Sprintf("%s.%s", tmpFile.Name(), ext)
	if err := os.Rename(tmpFile.Name(), tmpFileWithExt); err != nil {
		return &runResult{Runnable: rn, Retcode: -1, Error: err}
	}

	defer func() { _ = os.Remove(tmpFileWithExt) }()

	commandArgs := []string{}

	for _, s := range exe {
		if s == "$FILE" {
			commandArgs = append(commandArgs, tmpFileWithExt)
			continue
		}
		commandArgs = append(commandArgs, s)
	}

	cmd := exec.Command(commandArgs[0], commandArgs[1:]...)
	var (
		outBuf bytes.Buffer
		errBuf bytes.Buffer
	)

	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	r.log.WithFields(logrus.Fields{
		"command": commandArgs,
	}).Debug("running runnable")

	err = cmd.Run()
	res := &runResult{
		Runnable: rn,
		Retcode:  -1,
		Stdout:   outBuf.String(),
		Stderr:   errBuf.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.Success() {
			res.Retcode = 0
			return res
		}

		res.Error = err
		return res
	}

	res.Retcode = 0
	return res
}

type runResult struct {
	Runnable *Runnable
	Retcode  int
	Error    error
	Stdout   string
	Stderr   string
}

type Runnable struct {
	SourceFile string
	BlockStart string
	Lang       string
	LineOffset int
	Lines      []string
}

func (rn *Runnable) String() string {
	return strings.Join(rn.Lines, "\n")
}

func (rn *Runnable) Begin(lineno int, line string) {
	rn.Lines = []string{}
	rn.LineOffset = lineno - 1

	for i, char := range line {
		if char != '`' && char != '~' {
			rn.Lang = line[i+1:]
			rn.BlockStart = strings.TrimSpace(line[:i+1])
			return
		}
	}
}

type RunFilterFunc func(*Runnable) bool
