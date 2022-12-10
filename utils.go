package main

import (
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
