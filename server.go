package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
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

type PageData struct {
	Title       string
	StyleLink   string
	RootPath    string
	Index       []Link
	Breadcrumbs []Link
	// FriendlyCommit is a branch name, tag or the first 8 characters of the commit hash
	FriendlyCommit string
}

type PageContext struct {
	PageData
	Commit *object.Commit
	Repo   *git.Repository
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
			PageData: PageData{
				Title:       strings.TrimPrefix(projectPath, "/") + " - " + operation,
				StyleLink:   "/style.css",
				RootPath:    SettingServerPathPrefix,
				Breadcrumbs: makeBreadcrumbs(projectPath),
			},
		}
		pc.requireRepoOrList(w, projectPath)
		if pc.Repo == nil {
			return
		}

		// Operations that don't depend on commit
		switch operation {
		case "tags":
			fallthrough
		case "branches":
			pc.errorPageNotFound(w, operation+" not implemented yet")
		}
		pc.requireCommit(w, r)
		if pc.Commit == nil {
			return
		}
		switch operation {
		case "todo":
			pc.todoHandler(w, r)
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

func (pc PageContext) todoHandler(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		pc.todoListHandler(w, r)
		return
	}
}

func (pc *PageContext) todoListHandler(w http.ResponseWriter, r *http.Request) {
	todos, err := FindCommitTodos(*pc.Commit)
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
