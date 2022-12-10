package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"strings"
)

var (
	//go:embed templates/*
	wwwFS    embed.FS
	todoTmpl = template.Must(template.ParseFS(wwwFS, "templates/wrapper.tmpl.html", "templates/todo.tmpl.html"))
)

type PageData struct {
	Title     string
	StyleLink string
}

type TodoPageData struct {
	PageData
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
		switch operation {
		case "todo":
			todoHandler(w, r, projectPath)
		case "files":
			fallthrough
		case "log":
			fallthrough
		default:
			errorPageNotFound(w, operation+" does not exist")
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

func todoHandler(w http.ResponseWriter, r *http.Request, projectPath string) {
	commitHash := r.URL.Query().Get("commit")
	filename := r.URL.Query().Get("file")
	if filename == "" {
		todoListHandler(w, r, projectPath, commitHash)
		return
	}
}

func todoListHandler(w http.ResponseWriter, r *http.Request, projectPath, commitHash string) {
	repo := mustOpenGitRepo(w, projectPath)
	if repo == nil {
		return
	}
	hash, err := commitOrHead(repo, commitHash)
	if err != nil {
		errorPageServer(w, "failed to fetch HEAD", err)
		return
	}
	todos, err := FindCommitTodos(*hash)
	if err != nil {
		errorPageServer(w, "failed to find todos", err)
		return
	}
	//FindCommitTodos()
	data := TodoPageData{
		PageData: PageData{
			Title:     "test",
			StyleLink: "/style.css",
		},
		Todos: todos,
	}
	err = todoTmpl.Execute(w, data)
	if err != nil {
		log.Println("error in todo.tmpl.html:", err)
	}
}
