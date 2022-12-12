package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var (
	//go:embed templates/*
	wwwFS           embed.FS
	todoTmpl        = template.Must(template.ParseFS(wwwFS, "templates/wrapper.tmpl.html", "templates/todo.tmpl.html"))
	todoDetailsTmpl = template.Must(template.ParseFS(wwwFS, "templates/wrapper.tmpl.html", "templates/todo-details.tmpl.html"))
	errorTmpl       = template.Must(template.ParseFS(wwwFS, "templates/wrapper.tmpl.html", "templates/error.tmpl.html"))
	reposTmpl       = template.Must(template.ParseFS(wwwFS, "templates/wrapper.tmpl.html", "templates/repos.tmpl.html"))
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
	Operation      string
}

type PageContext struct {
	PageData
	projectPath string
	Commit      *object.Commit
	Repo        *git.Repository
}

type RepoPageData struct {
	PageData
	Repos []Link
}

type TodoPageData struct {
	PageData
	Todos []TodoDesc
}

type TodoDetailsData struct {
	PageData
	Todo TodoDesc
}

func serve() {
	log.Println("starting server at", SettingPort)
	http.ListenAndServe(SettingPort, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rpath := strings.TrimPrefix(r.URL.Path, SettingServerPathPrefix)
		if staticFile(w, r, rpath) {
			return
		}
		// General format of paths is <project path>[/-/<operation>]
		projectOperation := strings.Split(rpath, "/-/")
		projectPath := projectOperation[0]
		// TODO [use_files_as_default]: When file page is implemented, used that as default project page
		pc := PageContext{
			PageData: PageData{
				Title:       strings.TrimPrefix(projectPath, "/"),
				StyleLink:   "/style.css",
				RootPath:    SettingServerPathPrefix,
				Breadcrumbs: makeBreadcrumbs(projectPath),
			},
			projectPath: projectOperation[0],
		}
		if len(projectOperation) > 1 && projectOperation[1] != "" {
			pc.Operation = projectOperation[1]
			pc.Title += " - " + pc.Operation
		}
		pc.requireRepoOrList(w, projectPath)
		if pc.Repo == nil {
			return
		}

		// Operations that don't depend on commit
		switch pc.Operation {
		case "tags":
			fallthrough
		case "branches":
			pc.errorPageNotFound(w, pc.Operation+" not implemented yet")
		}
		pc.requireCommit(w, r)
		if pc.Commit == nil {
			return
		}
		switch pc.Operation {
		case "":
			fallthrough
		case "todo":
			pc.todoHandler(w, r)
		case "files":
			fallthrough
		case "log":
			fallthrough
		default:
			pc.errorPageNotFound(w, pc.Operation+" does not exist")
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

func (pc *PageContext) todoHandler(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	lineNum, err := strconv.Atoi(r.URL.Query().Get("line"))
	// Todo specified - get details page
	if filename != "" && err == nil {
		pc.todoDetailsHandler(w, r, filename, lineNum)
		return
	}
	// List all project TODOs
	todoMap := pc.requireCachedTodos(w, pc.projectPath, pc.Commit.Hash.String(), 3*time.Second)
	if todoMap == nil {
		return
	}
	todos := make([]TodoDesc, 0, len(todoMap))
	for _, t := range todoMap {
		todos = append(todos, t)
	}
	Sort(todos, []string{"pri", "id"})
	data := TodoPageData{
		PageData: pc.PageData,
		Todos:    todos,
	}
	logPageTmplErr("todo_list", todoTmpl.Execute(w, data))
}

func (pc *PageContext) todoDetailsHandler(w http.ResponseWriter, r *http.Request, filename string, lineNum int) {
	file, err := pc.Commit.File(filename)
	if err != nil {
		pc.errorPageNotFound(w, "file not found: "+filename)
		return
	}
	lines, err := getLines(pc.Commit, file)
	if err != nil {
		pc.errorPageServer(w, "failed to get contents of file:"+filename, err)
		return
	}
	tex := getTodoExtractor(filename)
	todo := tex.ExtractFull(lineNum, lines)
	if todo == nil {
		pc.errorPageNotFound(w, "no todo found at the given location")
		return
	}
	data := TodoDetailsData{
		PageData: pc.PageData,
		Todo:     *todo,
	}
	logPageTmplErr("todo_details", todoDetailsTmpl.Execute(w, data))
}
