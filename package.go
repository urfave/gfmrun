package gfmxr

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"gopkg.in/urfave/cli.v2"
)

var (
	VersionString   = ""
	RevisionString  = ""
	GeneratedString = ""
	CopyrightString = ""
)

func init() {
	cli.VersionPrinter = printVersion
}

func printVersion(_ *cli.Context) {
	fmt.Printf("%s\n", VersionString)
}

func getHomeDir() string {
	if v := os.Getenv("HOME"); v != "" {
		return v
	}

	curUser, err := user.Current()
	if err != nil {
		// well, sheesh
		return "."
	}

	return curUser.HomeDir
}

func getCacheDir() string {
	if xdgCacheHome := os.Getenv("XDG_CACHE_HOME"); xdgCacheHome != "" {
		return filepath.Join(xdgCacheHome, "gfmxr")
	}

	return filepath.Join(getHomeDir(), ".cache", "gfmxr")
}

type multiError []error

func (me multiError) Error() string {
	return fmt.Sprintf("%#v", me)
}

func (me multiError) Errors() []error {
	return []error(me)
}
