package main

import (
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type RepoInfo struct {
	Link
	Name        string
	Description string
}

var RegisteredRepos []RepoInfo

func init() {
	// TODO [overwrite_repo_register]: Make it possible to overwrite repo list, with env var or file.
	// Manually maintained lists, when implemented, should be the preferred way to use gitwood.
	// This WalkDir is just here to make it possible to use gitwood without any config or args.
	log.Println("no repo register found - searching in", SettingRootDir)
	err := filepath.WalkDir(SettingRootDir, func(fpath string, d fs.DirEntry, ierr error) error {
		if ierr != nil {
			log.Println("cannot open directory:", ierr)
			return fs.SkipDir
		}
		var isGitDir bool
		var desc []byte
		if d.IsDir() {
			isGitDir = d.Name() == ".git"
			desc, _ = os.ReadFile(path.Join(fpath, "description"))
		} else {
			// Bare repo
			isGitDir = d.Name() == "config"
			desc, _ = os.ReadFile(path.Join(filepath.Dir(fpath), "description"))
		}
		if isGitDir {
			fpath = filepath.Dir(fpath)
			_, ierr := git.PlainOpen(fpath)
			if ierr == nil {
				repoPath := strings.TrimPrefix(fpath, SettingRootDir)
				newRepo := RepoInfo{
					Name: filepath.Base(repoPath),
					Link: Link{
						Text: repoPath,
						Href: path.Join("/", SettingServerPathPrefix, repoPath),
					},
				}
				if len(desc) > 0 && !strings.HasPrefix(string(desc), "Unnamed repository") {
					newRepo.Description = string(desc)
				}
				RegisteredRepos = append(RegisteredRepos, newRepo)
			} else {
				log.Printf("failed to open %v: %v\n", fpath, ierr)
			}
			return fs.SkipDir
		}
		return nil
	})
	if err != nil {
		log.Fatal("failed to register git dirs:", err)
	}
	if len(RegisteredRepos) == 0 {
		log.Fatal("no repos registered")
	}
	log.Printf("found %v repositories", len(RegisteredRepos))
}

func (pc *PageContext) listReposWithPrefix(w http.ResponseWriter, pathPrefix string) {
	data := RepoPageData{
		PageData: pc.PageData,
		Repos:    []RepoInfo{},
	}
	for _, ri := range RegisteredRepos {
		if !strings.HasPrefix(ri.Text, pathPrefix) {
			continue
		}
		data.Repos = append(data.Repos, ri)
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

// requireCommit tries to load the commit with the given hash, if any.
// If no hash is given, HEAD is loaded.
// After loading the commit and setting it for the PageContext,
// all the relevant commit data is generated and cached.
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
		return
	}
	// Commit found - generate all relevant data for it
	generateAndCacheData(
		commitCacheKey(pc.projectPath, hash.String(), "todo"),
		func() (any, error) {
			return FindCommitTodos(*pc.Commit)
		},
	)
}
