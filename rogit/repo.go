package rogit

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
		return "0000000000000000000000000000000000000000"
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
	return "0000000000000000000000000000000000000000"
}

func (r Repo) Object(shasum string) (ObjectType, []byte, error) {
	if shasum == "" {
		shasum = r.HeadCommit()
	}
	return openObject(r.GitDir, shasum)
}

func (r Repo) Commit(shasum string) (*Commit, error) {
	otype, c, err := r.Object(shasum)
	if err != nil {
		return nil, err
	}
	if otype != OBJ_COMMIT {
		return nil, ErrNotACommit
	}
	return ParseCommit(shasum, string(c))
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
	for commit.Parent != "" {
		commit, err = r.Commit(commit.Parent)
		if err != nil {
			return commits, err
		}
		commits = append(commits, *commit)
	}
	return commits, nil
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
