package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
	"time"
)

func commitCacheKey(projectPath, commitHash, operation string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{projectPath, commitHash, "todo"}, "@")))
	return hex.EncodeToString(sum[:])[:SettingCacheHashSize]
}

func slashes(in string) string {
	in = "/" + strings.TrimPrefix(in, "/")
	return strings.TrimSuffix(in, "/") + "/"
}

func noSlashes(in string) string {
	return strings.TrimPrefix(strings.TrimSuffix(in, "/"), "/")
}

const (
	sinceMinute = time.Minute
	sinceHours  = time.Hour
	sinceDay    = 24 * sinceHours
	// Close enough
	sinceMonth = 31 * sinceDay
	sinceYear  = 365 * sinceDay
)

func prettyTime(ts time.Time) string {
	since := time.Since(ts)
	var num int
	var unit string
	switch {
	case since > sinceYear:
		num = int(since / sinceYear)
		unit = "year"
	case since > sinceMonth:
		num = int(since / sinceMonth)
		unit = "month"
	case since > sinceDay:
		num = int(since / sinceDay)
		unit = "day"
	case since > sinceHours:
		unit = "hour"
		num = int(since / sinceHours)
	case since > sinceMinute:
		unit = "minute"
		num = int(since / sinceMinute)
	default:
		return "just now"
	}
	if num >= 2 {
		unit += "s"
	}
	return fmt.Sprintf("%v %v ago", num, unit)
}

func makeBreadcrumbs(rpath string) []Link {
	var links []Link
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(rpath, "/"), "/"), "/")
	for i, part := range parts {
		l := Link{
			Text: part,
		}
		if i != len(parts)-1 {
			l.Href = path.Join("/", SettingServerPathPrefix, strings.Join(parts[:i+1], "/"))
		}
		links = append(links, l)
	}
	return links
}
