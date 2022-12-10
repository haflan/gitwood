package main

import (
	"errors"
	"log"
	"net/http"
	"path"
	"strings"

	"github.com/go-git/go-git/v5"
)

func mustOpenGitRepo(w http.ResponseWriter, projectPath string) *git.Repository {
	repo, err := git.PlainOpen(path.Join(SettingRootDir, projectPath))
	if err != nil {
		if errors.Is(err, git.ErrRepositoryNotExists) {
			projectPath = strings.TrimPrefix(projectPath, "/")
			message := "repository " + projectPath + " does not exist"
			if projectPath == "" {
				message = "no git repository in the server root directory"
			}
			errorPageNotFound(w, message)
		} else {
			errorPageServer(w, "unknown server error")
		}
		return nil
	}
	return repo
}

func errorPageNotFound(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusNotFound)
	// TODO [error_pages]: Create HTTP error pages that match the style of other pages
	w.Write([]byte(message))
}

func errorPageServer(w http.ResponseWriter, message string) {
	log.Println(message)
	w.WriteHeader(http.StatusInternalServerError)
	// error_pages
	w.Write([]byte(message))
}
