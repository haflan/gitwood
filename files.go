package main

import (
	_ "embed"
	"html/template"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type LogEntry struct {
	Hash      string
	Title     string
	Message   template.HTML
	Timestamp time.Time
}

type LogPageData struct {
	PageData
	Log []LogEntry
}

type CommitFile struct {
	Hash string
	Name string
	Size int64
	Mode filemode.FileMode
}

func ListCommitFiles(repo *git.Repository, hash plumbing.Hash) ([]CommitFile, error) {
	cob, err := repo.CommitObject(hash)
	if err != nil {
		return nil, err
	}
	fIter, err := cob.Files()
	if err != nil {
		return nil, err
	}
	var files []CommitFile
	fIter.ForEach(func(f *object.File) error {
		files = append(files, CommitFile{
			Hash: f.Hash.String(),
			Name: f.Name,
			Size: f.Size,
			Mode: f.Mode,
		})
		return nil
	})
	return files, nil
}
