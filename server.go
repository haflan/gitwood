package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

var (
	//go:embed templates/*
	wwwFS     embed.FS
	todoTmpl  = template.Must(template.ParseFS(wwwFS, "templates/wrapper.tmpl.html", "templates/todo.tmpl.html"))
	errorTmpl = template.Must(template.ParseFS(wwwFS, "templates/wrapper.tmpl.html", "templates/error.tmpl.html"))
)

type Link struct {
	Text string
	Href string
}

type PageContext struct {
	Title       string
	StyleLink   string
	RootPath    string
	Index       []Link
	Breadcrumbs []Link
	// Commit can be the hash of a commit, or a ref
	Commit string
}

type TodoPageData struct {
	PageContext
	Todos []TodoDesc
}

func serve() {
	http.ListenAndServe(":8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rpath := strings.TrimPrefix(r.URL.Path, SettingServerPathPrefix)
		if staticFile(w, r, rpath) {
			return
		}
		// General format of paths is <project path>[/-/<operation>]
		projectOperation := strings.Split(rpath, "/-/")
		projectPath := projectOperation[0]
		// TODO [use_files_as_default]: When file page is implemented, used that as default project page
		operation := "todo"
		if len(projectOperation) > 1 && projectOperation[1] != "" {
			operation = projectOperation[1]
		}

		pc := PageContext{
			Title:       strings.TrimPrefix(projectPath, "/") + " - todo",
			StyleLink:   "/style.css",
			RootPath:    SettingServerPathPrefix,
			Breadcrumbs: makeBreadcrumbs(projectPath),
			Commit:      r.URL.Query().Get("commit"),
		}
		switch operation {
		case "todo":
			pc.todoHandler(w, r, projectPath)
		case "files":
			fallthrough
		case "log":
			fallthrough
		default:
			pc.errorPageNotFound(w, operation+" does not exist")
		}
	}))
}

func staticFile(w http.ResponseWriter, r *http.Request, rpath string) bool {
	var out []byte
	var contentType string
	// Static
	switch rpath {
	case "/style.css":
		contentType = "text/css"
		out, _ = wwwFS.ReadFile("templates/style.css")
	default:
		return false
	}
	w.Header().Add("Content-Type", contentType)
	w.Write(out)
	return true
}

func (pc PageContext) todoHandler(w http.ResponseWriter, r *http.Request, projectPath string) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		pc.todoListHandler(w, r, projectPath, pc.Commit)
		return
	}
}

func (pc *PageContext) todoListHandler(w http.ResponseWriter, r *http.Request, projectPath, commit string) {
	// TODO [generalize_find_repo_commit]: 2022-12-10T22:33Z Generalize functionality for loading repo and commit.
	// This is relevant for most pages. Maybe make it part of the page context?
	// Only problem with that is that it's probably not very idiomatic to expose implementation details
	// to the templates, but meh for now.
	repo := pc.mustOpenGitRepo(w, projectPath)
	if repo == nil {
		return
	}
	if strings.HasPrefix(commit, "refs/heads/") {
		ref, err := repo.Reference(plumbing.ReferenceName(commit), true)
		if err != nil {
			pc.errorPageServer(w, "could not find the given commit", err)
			return
		}
		commit = ref.Hash().String()
		// Prettify commit for the page:
		pc.Commit = strings.TrimPrefix(pc.Commit, "refs/heads/")
	} else {
		pc.Commit = pc.Commit[:8]
	}
	hash, err := commitOrHead(repo, commit)
	if err != nil {
		pc.errorPageServer(w, "failed to fetch HEAD", err)
		return
	}

	todos, err := FindCommitTodos(*hash)
	if err != nil {
		pc.errorPageServer(w, "failed to find todos", err)
		return
	}
	//FindCommitTodos()
	data := TodoPageData{
		PageContext: *pc,
		Todos:       todos,
	}
	err = todoTmpl.Execute(w, data)
	if err != nil {
		log.Println("error in todo.tmpl.html:", err)
	}
}
