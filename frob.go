package gfmxr

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	DefaultFrobs = map[string]Frob{
		"bash":       NewSimpleInterpretedFrob("bash", "bash"),
		"go":         &GoFrob{},
		"java":       &JavaFrob{},
		"javascript": NewSimpleInterpretedFrob("js", "node"),
		"json":       NewSimpleInterpretedFrob("json", "node"),
		"python":     NewSimpleInterpretedFrob("py", "python"),
		"ruby":       NewSimpleInterpretedFrob("rb", "ruby"),
		"shell":      NewSimpleInterpretedFrob("bash", "bash"),
		"sh":         NewSimpleInterpretedFrob("sh", "sh"),
		"zsh":        NewSimpleInterpretedFrob("zsh", "zsh"),
	}

	errEmptySource = fmt.Errorf("empty source")

	javaPublicClassRe = regexp.MustCompile("public +class +([^ ]+)")
)

type Frob interface {
	Extension() string
	CanExecute(*Runnable) error
	TempFileName(*Runnable) string
	Environ(*Runnable) []string
	Commands(*Runnable) []*command
}

type command struct {
	Main bool
	Args []string
}

func NewSimpleInterpretedFrob(ext, interpreter string) Frob {
	return &InterpretedFrob{
		ext:  ext,
		env:  []string{},
		tmpl: []string{interpreter, "--", "{{.FILE}}"},
	}
}

type InterpretedFrob struct {
	ext  string
	env  []string
	tmpl []string
}

func (e *InterpretedFrob) Extension() string {
	return e.ext
}

func (e *InterpretedFrob) CanExecute(rn *Runnable) error {
	if len(rn.Lines) < 1 {
		return errEmptySource
	}
	return nil
}

func (e *InterpretedFrob) TempFileName(_ *Runnable) string {
	return fmt.Sprintf("example.%s", e.ext)
}

func (e *InterpretedFrob) Environ(_ *Runnable) []string {
	return e.env
}

func (e *InterpretedFrob) Commands(_ *Runnable) []*command {
	return []*command{
		&command{
			Main: true,
			Args: e.tmpl,
		},
	}
}

type GoFrob struct{}

func (e *GoFrob) Extension() string {
	return "go"
}

func (e *GoFrob) TempFileName(_ *Runnable) string {
	return "example.go"
}

func (e *GoFrob) CanExecute(rn *Runnable) error {
	if len(rn.Lines) < 1 {
		return errEmptySource
	}

	trimmedLine0 := strings.TrimSpace(rn.Lines[0])

	if trimmedLine0 != "package main" {
		return fmt.Errorf("first line is not \"package main\": %q", trimmedLine0)
	}

	return nil
}

func (e *GoFrob) Environ(_ *Runnable) []string {
	return []string{}
}

func (e *GoFrob) Commands(_ *Runnable) []*command {
	return []*command{
		&command{
			Args: []string{"go", "build", "-o", "{{.NAMEBASE}}" + os.Getenv("GOEXE"), "{{.FILE}}"},
		},
		&command{
			Main: true,
			Args: []string{"{{.NAMEBASE}}" + os.Getenv("GOEXE")},
		},
	}
}

type JavaFrob struct{}

func (e *JavaFrob) Extension() string {
	return "java"
}

func (e *JavaFrob) CanExecute(rn *Runnable) error {
	if len(rn.Lines) < 1 {
		return errEmptySource
	}

	for _, line := range rn.Lines {
		if javaPublicClassRe.MatchString(line) {
			return nil
		}
	}

	return fmt.Errorf("no public class found")
}

func (e *JavaFrob) TempFileName(rn *Runnable) string {
	return fmt.Sprintf("%s.java", e.getClassName(rn.String()))
}

func (e *JavaFrob) Environ(_ *Runnable) []string {
	return []string{}
}

func (e *JavaFrob) Commands(rn *Runnable) []*command {
	return []*command{
		&command{
			Args: []string{"javac", "{{.BASENAME}}"},
		},
		&command{
			Main: true,
			Args: []string{"java", e.getClassName(rn.String())},
		},
	}
}

func (e *JavaFrob) getClassName(source string) string {
	if m := javaPublicClassRe.FindStringSubmatch(source); len(m) > 1 {
		return m[1]
	}

	return "Unknown"
}
