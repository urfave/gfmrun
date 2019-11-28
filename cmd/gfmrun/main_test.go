package main

import (
	"os"
	"testing"

	"github.com/urfave/cli/v2"
)

func TestMain(m *testing.M) {
	stdout := os.Stdout
	stderr := os.Stderr
	os.Stdout, _ = os.Create(os.DevNull)
	os.Stderr, _ = os.Create(os.DevNull)
	cli.OsExiter = func(i int) { }

	defer func() {
		os.Stdout = stdout
		os.Stderr = stderr
		cli.OsExiter = os.Exit
	}()

	os.Args = []string{"gfmrun", "-h"}
	main()
}
