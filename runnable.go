package gfmrun

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/Sirupsen/logrus"
)

var (
	defaultKillDuration = time.Second * 3
	zeroDuration        = time.Second * 0

	wd, wdErr = os.Getwd()
)

type skipErr struct {
	Reason string
}

func (e *skipErr) Error() string {
	return fmt.Sprintf("skipped because %s", e.Reason)
}

func init() {
	if wdErr != nil {
		panic(fmt.Sprintf("unable to get working directory: %v", wdErr))
	}
}

type Runnable struct {
	Frob       Frob
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
	rn.Lang = strings.ToLower(strings.TrimSpace(codeGateCharsRe.ReplaceAllString(line, "")))
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

func (rn *Runnable) Args() []string {
	rn.parseTags()

	if v, ok := rn.Tags["args"]; ok {
		if iv, ok := v.([]interface{}); ok {
			slv := []string{}
			for _, v := range iv {
				slv = append(slv, v.(string))
			}
			return slv
		}
	}

	return nil
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

func (rn *Runnable) IsValidOS() bool {
	rn.parseTags()
	v, ok := rn.Tags["os"]
	if !ok {
		return true
	}

	switch v.(type) {
	case string:
		return runtime.GOOS == v.(string)
	case []interface{}:
		for _, s := range v.([]interface{}) {
			if runtime.GOOS == s.(string) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func (rn *Runnable) parseTags() {
	if rn.Tags == nil {
		rn.Tags = map[string]interface{}{}
	}

	if rn.RawTags == "" {
		return
	}

	err := json.Unmarshal([]byte(html.UnescapeString(rn.RawTags)), &rn.Tags)
	if err != nil {
		rn.log.WithField("err", err).Warn("failed to parse raw tags")
	}
}

func (rn *Runnable) Extract(i int, dir string) *runResult {
	if dir == "" {
		dir = "."
	}

	outFileName := filepath.Join(dir, fmt.Sprintf("%03d%s", i+1, rn.Frob.TempFileName(rn)))

	rn.log.WithFields(logrus.Fields{
		"filename": outFileName,
	}).Info("extracting example")

	err := ioutil.WriteFile(outFileName, []byte(rn.String()), os.FileMode(0600))
	if err != nil {
		return &runResult{Runnable: rn, Retcode: -1, Error: err}
	}

	return &runResult{Runnable: rn, Retcode: 0}
}

func (rn *Runnable) Run(i int) *runResult {
	if !rn.IsValidOS() {
		return &runResult{
			Runnable: rn,
			Retcode:  0,
			Error:    &skipErr{Reason: "os not supported"},
		}
	}

	if interruptable, _ := rn.Interruptable(); interruptable && runtime.GOOS == "windows" {
		return &runResult{
			Runnable: rn,
			Retcode:  0,
			Error:    &skipErr{Reason: "interrupt tag is not supported on windows"},
		}
	}

	tmpDir, err := ioutil.TempDir("", "gfmrun")
	if err != nil {
		return &runResult{Runnable: rn, Retcode: -1, Error: err}
	}

	defer func() {
		if os.Getenv("GFMRUN_PRESERVE_TMPFILES") == "1" {
			return
		}
		_ = os.RemoveAll(tmpDir)
	}()

	tmpFilename := rn.Frob.TempFileName(rn)
	tmpFile, err := os.Create(filepath.Join(tmpDir, tmpFilename))
	if err != nil {
		return &runResult{Runnable: rn, Retcode: -1, Error: err}
	}

	if _, err := tmpFile.Write([]byte(rn.String())); err != nil {
		return &runResult{Runnable: rn, Retcode: -1, Error: err}
	}

	if err := tmpFile.Close(); err != nil {
		return &runResult{Runnable: rn, Retcode: -1, Error: err}
	}

	nameBase := strings.Replace(tmpFile.Name(), "."+rn.Frob.Extension(), "", 1)

	expandedCommands := []*command{}

	tmplVars := map[string]string{
		"BASENAME": filepath.Base(tmpFile.Name()),
		"DIR":      tmpDir,
		"EXT":      rn.Frob.Extension(),
		"FILE":     tmpFile.Name(),
		"NAMEBASE": nameBase,
	}

	for _, c := range rn.Frob.Commands(rn) {
		expandedArgs := []string{}
		for _, s := range c.Args {
			buf := &bytes.Buffer{}
			err = template.Must(template.New("tmp").Parse(s)).Execute(buf, tmplVars)
			if err != nil {
				return &runResult{Runnable: rn, Retcode: -1, Error: err}
			}
			expandedArgs = append(expandedArgs, buf.String())
		}
		expandedCommands = append(expandedCommands,
			&command{
				Main: c.Main,
				Args: expandedArgs,
			})
	}

	env := os.Environ()
	env = append(env, rn.Frob.Environ(rn)...)
	env = append(env,
		fmt.Sprintf("GFMRUN_BASENAME=%s", filepath.Base(tmpFile.Name())),
		fmt.Sprintf("BASENAME=%s", filepath.Base(tmpFile.Name())),
		fmt.Sprintf("GFMRUN_DIR=%s", tmpDir),
		fmt.Sprintf("DIR=%s", tmpDir),
		fmt.Sprintf("GFMRUN_EXT=%s", rn.Frob.Extension()),
		fmt.Sprintf("EXT=%s", rn.Frob.Extension()),
		fmt.Sprintf("GFMRUN_FILE=%s", tmpFile.Name()),
		fmt.Sprintf("FILE=%s", tmpFile.Name()),
		fmt.Sprintf("GFMRUN_NAMEBASE=%s", nameBase),
		fmt.Sprintf("NAMEBASE=%s", nameBase))

	defer func() { _ = os.Chdir(wd) }()
	if err = os.Chdir(tmpDir); err != nil {
		return &runResult{Runnable: rn, Retcode: -1, Error: err}
	}

	return rn.executeCommands(env, expandedCommands)
}

func (rn *Runnable) executeCommands(env []string, commands []*command) *runResult {
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	var err error
	interruptable := false
	interrupted := false
	dur := defaultKillDuration

	rn.log.WithFields(logrus.Fields{
		"runnable": rn.GoString(),
	}).Debug("running runnable")

	for _, c := range commands {
		args := c.Args[1:]

		if tagArgs := rn.Args(); c.Main && tagArgs != nil {
			args = append(args, tagArgs...)
		}

		cmd := exec.Command(c.Args[0], args...)
		cmd.Env = env
		cmd.Stdout = outBuf
		cmd.Stderr = errBuf

		rn.log.WithFields(logrus.Fields{
			"command": c.Args,
		}).Debug("running runnable command")

		interruptable, dur = rn.Interruptable()

		if c.Main && interruptable {
			rn.log.WithFields(logrus.Fields{
				"cmd": cmd,
				"dur": dur,
			}).Debug("running with `Start`")

			err = cmd.Start()
			time.Sleep(dur)

			for _, sig := range []syscall.Signal{
				syscall.SIGINT,
				syscall.SIGHUP,
				syscall.SIGTERM,
				syscall.SIGKILL,
			} {
				if cmd.Process == nil {
					rn.log.WithFields(logrus.Fields{
						"signal": sig,
						"cmd":    cmd,
					}).Debug("breaking due to missing process")
					break
				}

				rn.log.WithFields(logrus.Fields{
					"signal": sig,
				}).Debug("attempting signal")

				sigErr := cmd.Process.Signal(sig)

				if sigErr != nil {
					rn.log.WithFields(logrus.Fields{
						"signal": sig,
						"err":    sigErr,
					}).Debug("signal returned error")

					time.Sleep(500 * time.Millisecond)
					continue
				}

				proc, _ := os.FindProcess(cmd.Process.Pid)
				sigErr = proc.Signal(syscall.Signal(0))
				if sigErr != nil && sigErr.Error() == "no such process" {
					interrupted = true
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
		} else if !c.Main {
			rn.log.WithField("cmd", cmd).Debug("running non-Main with `Run`")
			err = cmd.Run()
		} else {
			rn.log.WithField("cmd", cmd).Debug("running with `Run`")
			err = cmd.Run()
		}
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
			rn.log.WithFields(logrus.Fields{
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
