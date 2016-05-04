package gfmxr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"

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

	defaultKillDuration = time.Second * 3
	zeroDuration        = time.Second * 0

	rawTagsRe = regexp.MustCompile("<!-- *({.+}) *-->")
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

type mdState string

const (
	mdStateText      mdState = "text"
	mdStateCodeBlock mdState = "code-block"
	mdStateRunnable  mdState = "runnable"
	mdStateComment   mdState = "comment"
)

// custom markdown parser egad
// (because blackfriday doesn't give us line numbers (???))
func (r *Runner) findRunnables(i int, sourceName, source string) []*Runnable {
	runnables := []*Runnable{}
	cur := NewRunnable(sourceName, r.log)
	state := mdStateText
	lastLine := ""
	lastComment := ""

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
				lastComment = ""
				state = mdStateText
				continue
			}

			if state == mdStateRunnable && trimmedLine == cur.BlockStart {
				runnables = append(runnables, cur)
				cur = NewRunnable(sourceName, r.log)
				r.log.WithField("runnable_count", len(runnables)).Debug("leaving runnable code block")
				lastComment = ""
				state = mdStateText
				continue
			}

			unGated := strings.Replace(strings.Replace(trimmedLine, "`", "", -1), "~", "", -1)
			if len(unGated) > 0 {
				r.log.WithField("lineno", j).Debug("starting new runnable")
				trimmedComment := rawTagsRe.FindStringSubmatch(strings.TrimSpace(lastComment))
				if len(trimmedComment) > 1 {
					r.log.WithField("raw_tags", trimmedComment[1]).Debug("setting raw tags")
					cur.RawTags = trimmedComment[1]
				}
				cur.Begin(j, trimmedLine)
				state = mdStateRunnable
				continue
			}

			r.log.WithField("lineno", j).Debug("starting new non-runnable code block")
			lastComment = ""
			state = mdStateCodeBlock
		} else if strings.HasPrefix(trimmedLine, "<!--") && strings.HasSuffix(trimmedLine, "-->") {
			lastComment = line
			state = mdStateText
		} else if strings.HasPrefix(trimmedLine, "<!--") {
			lastComment = line
			state = mdStateComment
		} else if strings.HasPrefix(trimmedLine, "-->") {
			if state == mdStateComment {
				lastComment += line
				state = mdStateText
			}
		} else {
			if state == mdStateComment {
				lastComment += line
			}

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

	interruptable, dur := rn.Interruptable()
	interrupted := false

	if interruptable {
		r.log.WithFields(logrus.Fields{"cmd": cmd, "dur": dur}).Debug("running with `Start`")
		err = cmd.Start()
		<-time.After(dur)
		_ = cmd.Process.Signal(os.Interrupt)
		_ = cmd.Process.Signal(syscall.SIGHUP)
		if err == nil {
			_, err = cmd.Process.Wait()
		}
		interrupted = true
	} else {
		r.log.WithField("cmd", cmd).Debug("running with `Run`")
		err = cmd.Run()
	}

	res := &runResult{
		Runnable: rn,
		Retcode:  -1,
		Stdout:   outBuf.String(),
		Stderr:   errBuf.String(),
	}

	expectedOutput := rn.ExpectedOutput()

	if expectedOutput != nil {
		if !expectedOutput.MatchString(res.Stdout) {
			res.Error = fmt.Errorf("expected output does not match actual: %q != %q",
				expectedOutput, res.Stdout)
			return res
		} else {
			r.log.WithFields(logrus.Fields{
				"expected": fmt.Sprintf("%q", expectedOutput.String()),
				"actual":   fmt.Sprintf("%q", res.Stdout),
			}).Debug("output matched")
		}
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.Success() {
			res.Retcode = 0
			return res
		}

		res.Error = err
		if interrupted && interruptable {
			res.Error = nil
		}
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
	RawTags    string
	Tags       map[string]interface{}
	SourceFile string
	BlockStart string
	Lang       string
	LineOffset int
	Lines      []string

	log *logrus.Logger
}

func NewRunnable(sourceName string, log *logrus.Logger) *Runnable {
	return &Runnable{
		Tags:       map[string]interface{}{},
		Lines:      []string{},
		SourceFile: sourceName,

		log: log,
	}
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

func (rn *Runnable) Interruptable() (bool, time.Duration) {
	rn.parseTags()
	if v, ok := rn.Tags["interrupt"]; ok {
		if bv, ok := v.(bool); ok {
			return bv, defaultKillDuration
		}

		if sv, ok := v.(string); ok {
			if dv, err := time.ParseDuration(sv); err == nil {
				return true, dv
			}
		}

		return true, defaultKillDuration
	}

	return false, zeroDuration
}

func (rn *Runnable) ExpectedOutput() *regexp.Regexp {
	rn.parseTags()

	if v, ok := rn.Tags["output"]; ok {
		if s, ok := v.(string); ok {
			return regexp.MustCompile(s)
		}
	}

	return nil
}

func (rn *Runnable) parseTags() {
	if rn.Tags == nil {
		rn.Tags = map[string]interface{}{}
	}

	if rn.RawTags == "" {
		return
	}

	err := json.Unmarshal([]byte(rn.RawTags), &rn.Tags)
	if err != nil {
		rn.log.WithField("err", err).Warn("failed to parse raw tags")
	}
}

type RunFilterFunc func(*Runnable) bool
