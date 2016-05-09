package gfmxr

import (
	"os"
	"os/user"
	"path/filepath"
)

var (
	VersionString = "?"
)

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
