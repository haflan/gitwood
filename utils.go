package main

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

func (pc *PageContext) listReposWithPrefix(w http.ResponseWriter, pathPrefix string) {
	data := RepoPageData{
		PageData: pc.PageData,
		Repos:    []Link{},
	}
	for _, repoPath := range SettingRegisteredRepos {
		if strings.HasPrefix(repoPath, pathPrefix) {
			data.Repos = append(data.Repos, Link{
				Text: repoPath,
				Href: path.Join("/", SettingServerPathPrefix, repoPath),
			})
		}
	}
	if len(data.Repos) == 0 {
		pc.errorPageNotFound(w, "no repos matching the path: "+pathPrefix)
		return
	}
	reposTmpl.Execute(w, data)
}

// requireRepoOrList tries to extract a git repo given by repoPath.
// If no repo exists at the path, it looks for repos with the path as prefix.
func (pc *PageContext) requireRepoOrList(w http.ResponseWriter, repoPath string) {
	var err error
	dirPrefix := path.Join(SettingRootDir, repoPath)
	pc.Repo, err = git.PlainOpen(dirPrefix)
	// Any request to a non-repo should try to load the path as a directory
	if errors.Is(err, git.ErrRepositoryNotExists) {
		pc.listReposWithPrefix(w, repoPath)
	} else if err != nil {
		pc.errorPageServer(w, "unexpected error when trying to open repository", err)
	}
}

func (pc *PageContext) mustGetRef(w http.ResponseWriter, refName string) plumbing.Hash {
	pi := strings.LastIndex(refName, "/")
	if pi == len(refName)-1 {
		pc.errorRequest(w, "invalid ref: "+refName)
		return plumbing.ZeroHash
	}
	prefix := refName[:pi+1]
	ref, err := pc.Repo.Reference(plumbing.ReferenceName(refName), true)
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			pc.errorPageNotFound(w, "could not find the given ref: "+refName)
		} else {
			pc.errorPageServer(w, "failed to find the given ref: "+refName, err)
		}
		return plumbing.ZeroHash
	}
	pc.FriendlyCommit = strings.TrimPrefix(refName, prefix)
	return ref.Hash()
}

// ExtractCommit tries to load the commit with the given hash, if any.
// If no hash is given, HEAD is loaded.
func (pc *PageContext) requireCommit(w http.ResponseWriter, r *http.Request) {
	commit := r.URL.Query().Get("commit")
	var hash plumbing.Hash
	var err error
	switch {
	case strings.HasPrefix(commit, "refs/"):
		hash = pc.mustGetRef(w, commit)
	case commit == "":
		var ref *plumbing.Reference
		ref, err = pc.Repo.Head()
		if err != nil {
			pc.errorPageServer(w, "failed to find HEAD for repository", err)
			return
		}
		pc.FriendlyCommit = ref.Name().Short()
		hash = ref.Hash()
	default:
		hash = plumbing.NewHash(commit)
	}
	if hash.IsZero() {
		return
	}
	if pc.FriendlyCommit == "" {
		pc.FriendlyCommit = commit[:8]
	}
	pc.Commit, err = pc.Repo.CommitObject(hash)
	if err != nil {
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			pc.errorPageNotFound(w, "no such commit: "+hash.String())
		} else {
			pc.errorPageServer(w, "failed to fetch commit with hash: "+hash.String(), err)
		}
		pc.Commit = nil // Just to be sure
	}
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
