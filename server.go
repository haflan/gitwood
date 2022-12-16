package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"path"
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
	ProjectLink    string
	Operation      string
	Resource       string
}

type PageContext struct {
	PageData
	projectPath string
	Commit      *object.Commit
	Repo        *git.Repository
}

type RepoPageData struct {
	PageData
	Repos []RepoInfo
}

type TodoPageData struct {
	PageData
	Todos []TodoDesc
}

type TodoDetailsData struct {
	PageData
	Todo            TodoDesc
	RenderedDetails template.HTML
}

func serve() {
	log.Println("starting server at", SettingPort)
	http.ListenAndServe(SettingPort, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rpath := strings.TrimPrefix(r.URL.Path, SettingServerPathPrefix)
		if staticFile(w, r, rpath) {
			return
		}
		// General format of paths is <project path>[/-/<operation>][/<resource>]
		projectPathOperation := strings.Split(rpath, "/-/")
		projectPath := projectPathOperation[0]
		// TODO [use_files_as_default]: When file page is implemented, used that as default project page
		pc := PageContext{
			PageData: PageData{
				Title:       "gitwood",
				StyleLink:   "/style.css",
				RootPath:    SettingServerPathPrefix,
				Breadcrumbs: makeBreadcrumbs(projectPath),
				ProjectLink: slashes(projectPath),
			},
			projectPath: projectPath,
		}
		if len(projectPathOperation) > 1 && projectPathOperation[1] != "" {
			op := strings.Split(projectPathOperation[1], "/")
			pc.Operation = op[0]
			if len(op) > 1 && op[1] != "" {
				pc.Resource = op[1]
			}
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
	// List all project TODOs
	todoMap := pc.requireCachedTodos(w, pc.projectPath, pc.Commit.Hash.String(), 3*time.Second)
	if todoMap == nil {
		return
	}
	// Will probably use path param instead, as mentioned in #todo_use_id_mapping.
	id := r.URL.Query().Get("id")
	if id != "" {
		pc.todoDetailsHandler(w, r, todoMap, id)
	} else {
		pc.todoListHandler(w, r, todoMap)
	}
}

func (pc *PageContext) todoListHandler(w http.ResponseWriter, r *http.Request, todoMap map[string]TodoDesc) {
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

func (pc *PageContext) todoDetailsHandler(w http.ResponseWriter, r *http.Request, todoMap map[string]TodoDesc, id string) {
	todo, ok := todoMap[id]
	if !ok {
		pc.errorPageNotFound(w, "no todo with id: "+id)
		return
	}
	file, err := pc.Commit.File(todo.FileName)
	if err != nil {
		pc.errorPageNotFound(w, "file not found: "+todo.FileName)
		return
	}
	lines, err := getLines(pc.Commit, file)
	if err != nil {
		pc.errorPageServer(w, "failed to get contents of file: "+todo.FileName, err)
		return
	}
	// NOTE [persistent_and_cache]: Caching full Todo remains
	tex := getTodoExtractor(todo.FileName)
	fullTodo := tex.ExtractFull(todo.Line, lines)
	if fullTodo == nil {
		pc.errorPageNotFound(w, "no todo found at the given location")
		return
	}
	todoRefs := map[string]string{}
	for tr := range todoMap {
		todoRefs[tr] = path.Join("/"+pc.RootPath, pc.projectPath, "-", "todo") + "?id=" + tr
	}
	data := TodoDetailsData{
		PageData:        pc.PageData,
		Todo:            *fullTodo,
		RenderedDetails: markdownToHTML(todoRefs, fullTodo.Details),
	}
	logPageTmplErr("todo_details", todoDetailsTmpl.Execute(w, data))
}
