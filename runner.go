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
		"go": func(rn *Runnable) (bool, string) {
			if len(rn.Lines) < 1 {
				return false, "empty source"
			}

			trimmedLine0 := strings.TrimSpace(rn.Lines[0])

			if trimmedLine0 != "package main" {
				return false, fmt.Sprintf("first line is not \"package main\": %q", trimmedLine0)
			}

			return true, ""
		},
		"python": func(_ *Runnable) (bool, string) { return true, "" },
	}
	DefaultExecutors = map[string][]string{
		"go": []string{"sh", "-c", "go build -o $FILE-goexe $FILE && exec $FILE-goexe"},
	}
	DefaultFileExtensions = map[string]string{
		"go":     "go",
		"python": "py",
	}

	defaultKillDuration = time.Second * 3
	zeroDuration        = time.Second * 0

	rawTagsRe = regexp.MustCompile("<!-- *({.+}) *-->")

	codeGateCharsRe = regexp.MustCompile("[`~]+")
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

func (r *Runner) findRunnables(i int, sourceName, source string) []*Runnable {
	finder := newRunnableFinder(sourceName, source, r.log)
	runnables := finder.Find()

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

		if ok, reason := ff(runnable); !ok {
			r.log.WithFields(logrus.Fields{
				"source": runnable.SourceFile,
				"lineno": runnable.LineOffset,
				"reason": reason,
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

	defer func() {
		if os.Getenv("GFMXR_PRESERVE_TMPFILES") == "1" {
			return
		}
		_ = os.Remove(tmpFileWithExt)
		_ = os.Remove(fmt.Sprintf("%s-goexe", tmpFileWithExt))
	}()

	commandArgs := []string{}

	for _, s := range exe {
		commandArgs = append(commandArgs, strings.Replace(s, "$FILE", tmpFileWithExt, -1))
	}

	cmd := exec.Command(commandArgs[0], commandArgs[1:]...)
	var (
		outBuf bytes.Buffer
		errBuf bytes.Buffer
	)

	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	r.log.WithFields(logrus.Fields{
		"command":  commandArgs,
		"runnable": rn.GoString(),
	}).Debug("running runnable")

	interruptable, dur := rn.Interruptable()
	interrupted := false

	if interruptable {
		r.log.WithFields(logrus.Fields{"cmd": cmd, "dur": dur}).Debug("running with `Start`")
		err = cmd.Start()

		for _, sig := range []syscall.Signal{
			syscall.SIGINT,
			syscall.SIGHUP,
			syscall.SIGTERM,
			syscall.SIGKILL,
		} {
			<-time.After(dur)
			r.log.WithFields(logrus.Fields{
				"signal": sig,
			}).Debug("attempting signal")

			sigErr := cmd.Process.Signal(sig)
			if sigErr != nil {
				r.log.WithFields(logrus.Fields{
					"signal": sig,
					"err":    sigErr,
				}).Debug("signal returned error")
				continue
			}

			proc, _ := os.FindProcess(cmd.Process.Pid)
			sigErr = proc.Signal(syscall.Signal(0))
			if sigErr != nil && sigErr.Error() == "no such process" {
				interrupted = true
				break
			}
		}
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

type mdState int

func (s mdState) String() string {
	switch s {
	case mdStateText:
		return "text"
	case mdStateCodeBlock:
		return "code-block"
	case mdStateRunnable:
		return "runnable"
	case mdStateComment:
		return "comment"
	default:
		return "unknown"
	}
}

const (
	mdStateText mdState = iota
	mdStateCodeBlock
	mdStateRunnable
	mdStateComment
)

var (
	mdStateTransTextCodeBlock     = calcStateTransition(mdStateText, mdStateCodeBlock)
	mdStateTransTextRunnable      = calcStateTransition(mdStateText, mdStateRunnable)
	mdStateTransTextComment       = calcStateTransition(mdStateText, mdStateComment)
	mdStateTransCodeBlockText     = calcStateTransition(mdStateCodeBlock, mdStateText)
	mdStateTransCodeBlockRunnable = calcStateTransition(mdStateCodeBlock, mdStateRunnable)
	mdStateTransCodeBlockComment  = calcStateTransition(mdStateCodeBlock, mdStateComment)
	mdStateTransRunnableText      = calcStateTransition(mdStateRunnable, mdStateText)
	mdStateTransRunnableCodeBlock = calcStateTransition(mdStateRunnable, mdStateCodeBlock)
	mdStateTransRunnableComment   = calcStateTransition(mdStateRunnable, mdStateComment)
	mdStateTransCommentText       = calcStateTransition(mdStateComment, mdStateText)
	mdStateTransCommentCodeBlock  = calcStateTransition(mdStateComment, mdStateCodeBlock)
	mdStateTransCommentRunnable   = calcStateTransition(mdStateComment, mdStateRunnable)
)

func calcStateTransition(a, b mdState) int {
	return int(a | (b << 2))
}

type runnableFinder struct {
	sourceName string
	source     string
	log        *logrus.Logger

	state mdState

	cur *Runnable

	line           string
	trimmedLine    string
	lineno         int
	textSize       int
	lastLine       string
	lastComment    string
	codeBlockStart string
}

// custom markdown scanner thingy egad
// (because blackfriday doesn't have all the things and/or I'm horrible)
func newRunnableFinder(sourceName, source string, log *logrus.Logger) *runnableFinder {
	rf := &runnableFinder{sourceName: sourceName, source: source, log: log}
	rf.reset()
	return rf
}

func (rf *runnableFinder) reset() {
	rf.cur = NewRunnable(rf.sourceName, rf.log)
	rf.state = mdStateText
	rf.line = ""
	rf.trimmedLine = ""
	rf.codeBlockStart = ""
	rf.lineno = 0
	rf.lastLine = ""
	rf.lastComment = ""
}

func (rf *runnableFinder) Find() []*Runnable {
	rf.reset()
	runnables := []*Runnable{}

	for j, line := range strings.Split(rf.source, "\n") {
		rf.line = line
		rf.lineno = j
		rf.lastLine = line
		rf.trimmedLine = strings.TrimSpace(rf.line)

		runnable := rf.handleLine()
		if runnable != nil {
			runnables = append(runnables, runnable)
			rf.log.WithField("runnable_count", len(runnables)).Debug("leaving runnable code block")
		}

		rf.log.WithFields(logrus.Fields{
			"source_name": rf.sourceName,
			"lineno":      rf.lineno,
			"line":        fmt.Sprintf("%q", rf.trimmedLine),
			"state":       rf.state,
		}).Debug("scanning")
	}

	if rf.state == mdStateRunnable {
		// whatever, let's give it a shot
		rf.cur.Lines = append(rf.cur.Lines, rf.lastLine)
		runnables = append(runnables, rf.cur)
	}

	return runnables
}

func (rf *runnableFinder) handleLine() *Runnable {
	if strings.HasPrefix(rf.trimmedLine, "```") || strings.HasPrefix(rf.trimmedLine, "~~~") {
		if rf.state == mdStateCodeBlock {
			if rf.trimmedLine == rf.codeBlockStart {
				return rf.setState(mdStateText)
			} else {
				rf.log.Debug("assuming nested code block")
				return nil
			}
		}

		if rf.state == mdStateRunnable {
			if rf.trimmedLine == rf.cur.BlockStart {
				// assuming this is the matching closing gate of the runnable code block
				return rf.setState(mdStateText)
			} else {
				rf.log.WithFields(logrus.Fields{
					"block_start": rf.cur.BlockStart,
					"lang":        rf.cur.Lang,
					"line":        rf.trimmedLine,
				}).Debug("mismatched closing gate")
			}
		}

		if len(codeGateCharsRe.ReplaceAllString(rf.trimmedLine, "")) > 0 {
			return rf.setState(mdStateRunnable)
		}

		return rf.setState(mdStateCodeBlock)
	} else if strings.HasPrefix(rf.trimmedLine, "<!--") && strings.HasSuffix(rf.trimmedLine, "-->") {
		if rf.state == mdStateText {
			rf.setState(mdStateComment)
			rf.line = ""
			rf.trimmedLine = ""
			rf.setState(mdStateText)
		} else {
			rf.log.WithFields(logrus.Fields{
				"state": rf.state,
				"line":  rf.trimmedLine,
			}).Debug("not setting lastComment")
		}
	} else if strings.HasPrefix(rf.trimmedLine, "<!--") {
		return rf.setState(mdStateComment)
	} else if strings.HasPrefix(rf.trimmedLine, "-->") && rf.state == mdStateComment {
		rf.trimmedLine = strings.TrimSpace(strings.Replace(rf.trimmedLine, "-->", "", 1))
		return rf.setState(mdStateText)
	}

	rf.handleLineInState()
	return nil
}

func (rf *runnableFinder) setState(newState mdState) *Runnable {
	oldState := rf.state
	rf.state = newState

	transition := calcStateTransition(oldState, newState)
	rf.log.WithFields(logrus.Fields{
		"transition": transition,
		"old_state":  oldState,
		"new_state":  newState,
	}).Debug("setting state")

	return rf.handleTransition(transition)
}

func (rf *runnableFinder) handleTransition(transition int) *Runnable {
	switch transition {
	case mdStateTransCodeBlockComment:
		rf.log.WithFields(logrus.Fields{
			"state":         mdStateCodeBlock,
			"invalid_state": mdStateComment,
		}).Debug("ignoring transition")
		rf.state = mdStateCodeBlock
	case mdStateTransRunnableCodeBlock:
		rf.log.WithFields(logrus.Fields{
			"state":         mdStateRunnable,
			"invalid_state": mdStateCodeBlock,
		}).Debug("ignoring transition")
		rf.state = mdStateRunnable
	case mdStateTransCodeBlockRunnable:
		rf.log.WithFields(logrus.Fields{
			"state":         mdStateCodeBlock,
			"invalid_state": mdStateRunnable,
		}).Debug("ignoring transition")
		rf.state = mdStateCodeBlock
	case mdStateTransTextCodeBlock:
		rf.codeBlockStart = rf.trimmedLine
		rf.textSize = 0
	case mdStateTransTextComment:
		rf.textSize = 0
		rf.lastComment = rf.line
	case mdStateTransCommentText:
		rf.textSize = len(rf.trimmedLine)
	case mdStateTransCodeBlockText:
		rf.codeBlockStart = ""
		rf.log.Debug("leaving non-runnable code block")
	case mdStateTransRunnableText:
		runnable := rf.cur
		rf.cur = NewRunnable(rf.sourceName, rf.log)
		rf.lastComment = ""
		return runnable
	case mdStateTransTextCodeBlock:
		rf.log.WithField("lineno", rf.lineno).Debug("starting new non-runnable code block")
		rf.lastComment = ""
	case mdStateTransTextRunnable, mdStateTransCommentRunnable:
		rf.log.WithFields(logrus.Fields{
			"lineno":       rf.lineno,
			"text_size":    rf.textSize,
			"last_comment": rf.lastComment,
		}).Debug("starting new runnable")

		// textSize of 0 means that the last comment is adjacent to the runnable
		if rf.textSize == 0 {
			trimmedComment := rawTagsRe.FindStringSubmatch(strings.TrimSpace(rf.lastComment))
			if len(trimmedComment) > 1 {
				rf.log.WithField("raw_tags", trimmedComment[1]).Debug("setting raw tags")
				rf.cur.RawTags = trimmedComment[1]
			}
		}

		rf.lastComment = ""
		rf.cur.Begin(rf.lineno, rf.trimmedLine)
	default:
		rf.log.WithField("transition", transition).Debug("unhandled transition")
	}
	return nil
}

func (rf *runnableFinder) handleLineInState() {
	switch rf.state {
	case mdStateComment:
		rf.lastComment += rf.line
	case mdStateRunnable:
		rf.cur.Lines = append(rf.cur.Lines, rf.line)
	case mdStateText:
		rf.textSize += len(rf.trimmedLine)
	}
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

func (rn *Runnable) GoString() string {
	rn.parseTags()
	return fmt.Sprintf("\nsource: %s:%d\ntags: %#v\nlang: %q\n\n%s\n",
		rn.SourceFile, rn.LineOffset, rn.Tags, rn.Lang, strings.Join(rn.Lines, "\n"))
}

func (rn *Runnable) Begin(lineno int, line string) {
	rn.Lines = []string{}
	rn.LineOffset = lineno + 1
	rn.Lang = strings.TrimSpace(codeGateCharsRe.ReplaceAllString(line, ""))
	rn.BlockStart = strings.TrimSpace(strings.Replace(line, rn.Lang, "", 1))
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

type RunFilterFunc func(*Runnable) (bool, string)
