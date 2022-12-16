package main

import (
	"os"
	"strings"
)

func envOrDefault(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		val = defaultVal
	}
	return val
}

var (
	SettingRootDir          = os.Getenv("PWD")
	SettingServerPathPrefix = strings.TrimPrefix(strings.TrimSuffix(os.Getenv("GITWOOD_PREFIX"), "/"), "/")
	SettingCacheHashSize    = 8
	SettingPort             = ":" + envOrDefault("GITWOOD_PORT", "8750")
)
