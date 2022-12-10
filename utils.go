package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// commitOrHead tries to load the commit with the given hash, if any.
// If no hash is given, HEAD is loaded.
func commitOrHead(repo *git.Repository, commitHash string) (*object.Commit, error) {
	var hash plumbing.Hash
	if commitHash != "" {
		hash = plumbing.NewHash(commitHash)
	} else {
		ref, err := repo.Head()
		if err != nil {
			return nil, err
		}
		hash = ref.Hash()
	}
	cob, err := repo.CommitObject(hash)
	if err != nil {
		return nil, err
	}
	return cob, nil
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
		unit = "hours"
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

func makeBreadcrumbs(path string) []Link {
	var links []Link
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/"), "/")
	for i, part := range parts {
		l := Link{
			Text: part,
		}
		if i != len(parts)-1 {
			l.Href = "/" + SettingServerPathPrefix + strings.Join(parts[:i+1], "/")
		}
		links = append(links, l)
	}
	return links
}
