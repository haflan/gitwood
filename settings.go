package main

import "os"

func envOrDefault(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		val = defaultVal
	}
	return val
}

var (
	SettingRootDir          = os.Getenv("PWD")
	SettingServerPathPrefix = os.Getenv("GITWOOD_PREFIX")
	SettingRegisteredRepos  []string
	SettingCacheHashSize    = 8
	SettingPort             = ":" + envOrDefault("GITWOOD_PORT", "8750")
)
