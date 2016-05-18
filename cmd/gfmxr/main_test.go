package main

import (
	"os"
	"testing"

	"gopkg.in/codegangsta/cli.v2"
)

func TestMain(t *testing.T) {
	stdout := os.Stdout
	stderr := os.Stderr
	os.Stdout, _ = os.Create(os.DevNull)
	os.Stderr, _ = os.Create(os.DevNull)
	cli.OsExiter = func(i int) { return }

	defer func() {
		os.Stdout = stdout
		os.Stderr = stderr
		cli.OsExiter = os.Exit
	}()

	os.Args = []string{"gfmxr", "-h"}
	main()
}
