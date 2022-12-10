package main

import (
	_ "embed"
	"fmt"
	"os"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func must(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type LogEntry struct {
	Hash      string
	Message   string
	Timestamp time.Time
}

type LogPageData struct {
	Title string
	Log   []LogEntry
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

// Old stuff
//func gitGen() {
//	if len(os.Args) < 2 {
//		fmt.Println("Use: skoggen <git repo>")
//		return
//	}
//	var err error
//	var logTemplate *template.Template
//	logTemplatePath := os.Getenv("GITWOOD_LOG_TEMPLATE")
//	if logTemplatePath == "" {
//		logTemplate, err = template.New("log").Parse(defaultLogTemplate)
//	} else {
//		logTemplate, err = template.New("log").ParseFiles(logTemplatePath)
//	}
//	must(err)
//	repoPath := os.Args[1]
//	repo, err := git.PlainOpen(repoPath)
//	must(err)
//	ref, err := repo.Head()
//	must(err)
//	cob, err := repo.CommitObject(ref.Hash())
//	must(err)
//	fIter, err := cob.Files()
//	must(err)
//	var files []CommitFile
//	fIter.ForEach(func(f *object.File) error {
//		files = append(files, CommitFile{
//			Hash: f.Hash.String(),
//			Name: f.Name,
//			Size: f.Size,
//			Mode: f.Mode,
//		})
//		return nil
//	})
//	var log []LogEntry
//	cIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
//	must(err)
//	err = cIter.ForEach(func(c *object.Commit) error {
//		log = append(log, LogEntry{
//			Timestamp: c.Committer.When,
//			Hash:      c.Hash.String(),
//			Message:   strings.TrimSpace(c.Message),
//		})
//		return nil
//	})
//	must(err)
//	logTemplate.Execute(os.Stdout, LogPageData{
//		Title: path.Base(repoPath),
//		Log:   log,
//	})
//}
