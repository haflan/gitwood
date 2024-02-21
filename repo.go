package gitwood

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type Repo struct {
	Head   string
	GitDir string
}

func (r Repo) String() string {
	return fmt.Sprintf("%v @%v", r.GitDir, r.Head)
}

func (r Repo) HeadCommit() string {
	fields := strings.Fields(r.Head)
	if len(fields) == 0 || fields[0] == "" {
		return NULL_HASH
	}
	if len(fields[0]) == 40 {
		return fields[0]
	}
	// Resolve ref
	if strings.HasPrefix(r.Head, "ref: ") {
		ref := fields[1]
		// First try the refs directory...
		hash, err := os.ReadFile(path.Join(r.GitDir, ref))
		if err == nil {
			return strings.TrimSpace(string(hash))
		}
		// ...then try info/refs if that fails,
		// because sometimes it's stored there apparently (haven't found details about this yet).
		// Update: git update-server-info writes refs to the info/refs file,
		// kind of as a branch *index* for dumb HTTP servers, so they don't have to traverse the refs directory.
		// AFAIU `refs/head/` should always exist, though, so I'm not sure what I was struggling with previously.
		// Maybe I just saw the info/refs file and thought it was the only place where refs are stored.
		infoRefs, err := os.ReadFile(path.Join(r.GitDir, "info/refs"))
		if err == nil {
			for _, line := range strings.Split(string(infoRefs), "\n") {
				fields = strings.Fields(line)
				if len(fields) >= 2 && fields[1] == ref {
					return fields[0]
				}
			}
		}
	}
	return NULL_HASH
}

func (r Repo) Object(shasum string) (ObjectType, []byte, error) {
	// Wrong place to do this! Don't even know if the caller wants a commit object.
	if shasum == "" {
		shasum = r.HeadCommit()
	}
	otype, o, err := r.openObject(shasum)
	if err != nil {
		return OBJ_INVALID, nil, fmt.Errorf("failed to open object %v: %w", shasum, err)
	}
	return otype, o, nil
}

func (r Repo) Log(shasum string) ([]Commit, error) {
	if shasum == "" {
		shasum = r.HeadCommit()
	}
	commit, err := r.Commit(shasum)
	if err != nil {
		return nil, err
	}
	commits := []Commit{*commit}
	for len(commit.Parents) > 0 {
		commit, err = r.Commit(commit.Parents[0])
		if err != nil {
			return commits, err
		}
		commits = append(commits, *commit)
	}
	return commits, nil
}

func (r Repo) WalkToPath(commitSum, path string, tw TreeWalker) (ObjectType, []byte, error) {
	if commitSum == "" {
		commitSum = r.HeadCommit()
	}
	commit, err := r.Commit(commitSum)
	if err != nil {
		return OBJ_INVALID, nil, err
	}
	return commit.WalkToPath(path, tw)
}

func newRepo(gitdir string, head []byte) *Repo {
	return &Repo{GitDir: gitdir, Head: strings.TrimSpace(string(head))}
}

func Open(gitdir string) (*Repo, error) {
	// First check if the given path is a git dir
	head, err := os.ReadFile(path.Join(gitdir, "HEAD"))
	if err == nil {
		return newRepo(gitdir, head), nil
	}
	// Then search for a .git dir or file
	fi, err := os.Stat(path.Join(gitdir, ".git"))
	if err != nil {
		return nil, err
	}
	// If .git is a directory, check if it contains the HEAD file
	if fi.IsDir() {
		gitdir = path.Join(gitdir, ".git")
		head, err = os.ReadFile(path.Join(gitdir, "HEAD"))
		if err != nil {
			return nil, err
		}
		return newRepo(gitdir, head), nil
	}
	// If '.git' is a file, the given dir is probably a submodule
	gitContents, err := os.ReadFile(path.Join(gitdir, ".git"))
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(gitContents), "\n") {
		f := strings.Fields(line)
		if f[0] == "gitdir:" {
			gitdir, err = filepath.Abs(path.Join(gitdir, f[1]))
			if err != nil {
				return nil, err
			}
			break
		}
	}
	head, err = os.ReadFile(path.Join(gitdir, "HEAD"))
	if err != nil {
		return nil, err
	}
	return newRepo(gitdir, head), nil
}
