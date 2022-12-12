package main

import "os"

var (
	SettingRootDir          = os.Getenv("PWD")
	SettingServerPathPrefix = os.Getenv("GITWOOD_PREFIX")
	SettingRegisteredRepos  []string
	SettingCacheHashSize    = 8
)
