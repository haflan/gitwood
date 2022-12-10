package main

import (
	"flag"
	"fmt"
	"html/template"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	texttemplate "text/template"

	_ "embed"

	"github.com/go-git/go-git/v5"
)

//go:embed templates/todos.html
var defaultHTMLTemplate string

//go:embed templates/todos.adoc
var defaultAdocTemplate string

type DrawableTodo struct {
	EvenRow bool
	T       Todo
}

type TodoInfo struct {
	Title string
	Show  TodoFieldsShown
	//Todos     []gen.TodoDesc
	//FileTodos map[string][]gen.TodoDesc
	Todos []DrawableTodo
}

type Template interface {
	Execute(io.Writer, any) error
}

const helpFields = `
  Fields:
    desc  Description
    file  File name
    line  File line number
    pri   Priority
    ref   Reference
    ts    Timestamp
`

func loadTemplate(templatePath string) Template {
	if templatePath == "" {
		templatePath = os.Getenv("SKOGGEN_TEMPLATE")
	}
	if templatePath == "" {
		if os.Getenv("SKOGGEN_MODE") == "adoc" {
			t := template.New("todoadoc")
			return template.Must(t.Parse(defaultAdocTemplate))
		} else {
			t := template.New("todohtml")
			return template.Must(t.Parse(defaultHTMLTemplate))
		}
	}
	templateBytes, err := os.ReadFile(templatePath)
	if err != nil {
		fmt.Println("couldn't load template file:", err)
		return nil
	}
	var temp Template
	if strings.HasSuffix(templatePath, ".html") {
		t := template.New("todohtml")
		temp, err = t.Parse(string(templateBytes))
	} else {
		t := texttemplate.New("todo")
		temp, err = t.Parse(string(templateBytes))
	}
	if err != nil {
		fmt.Println("failed to parse template:", err)
		return nil
	}
	return temp
}

func main() {
	var (
		fSort         string
		fHide         string
		fShow         string
		fFilter       string
		fTemplatePath string
	)
	flag.StringVar(&fSort, "sort", "", "comma separated list of fields to sort by")
	flag.StringVar(&fFilter, "filter", "", "comma separated list of fields that are required for todo to be included")
	flag.StringVar(&fHide, "hide", "", "comma separated list of fields that should be hidden (default none)")
	flag.StringVar(&fShow, "show", "", "comma separated list of fields that should be shown (default all)")
	flag.StringVar(&fTemplatePath, "temp", "", "path to template file (defaults to using embedded template)")
	flag.Parse()

	var (
		dir string
		err error
	)
	if len(flag.Args()) > 0 {
		dir, err = filepath.Abs(flag.Arg(0))
	} else {
		dir, err = filepath.Abs(".")
	}
	if err != nil {
		fmt.Println("Failed to find repo:", err)
		return
	}
	temp := loadTemplate(fTemplatePath)
	if temp == nil {
		return
	}
	repoPath := path.Base(dir)
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		fmt.Println("Failed to open repo:", err)
		return
	}
	ref, err := repo.Head()
	if err != nil {
		fmt.Println("Failed to find HEAD:", err)
		return
	}
	files, err := ListCommitFiles(repo, ref.Hash())
	if err != nil {
		fmt.Println("Failed to list files:", err)
		return
	}

	// Legacy stuff that may be relevant (for inspiration) still
	// todos, err := FindRepoTodos(dir)
	// if err != nil {
	// 	fmt.Println("Failed to get todos in repo:", err)
	// 	return
	// }
	// var fieldsShown TodoFieldsShown
	// if fHide != "" {
	// 	fieldsShown = MakeFieldsShownFromHidden(fHide)
	// } else {
	// 	fieldsShown = MakeFieldsShown(fShow)
	// }
	// //for _, td := range todos {
	// //	info.FileTodos[td.FileName] = append(info.FileTodos[td.FileName], td)
	// //}
	// todos = Filter(todos, strings.Split(fFilter, ","))
	// Sort(todos, strings.Split(fSort, ","))
	// var drawableTodos []DrawableTodo
	// for i, todo := range todos {
	// 	drawableTodos = append(drawableTodos, DrawableTodo{
	// 		EvenRow: i%2 == 0,
	// 		T:       todo,
	// 	})
	// }
	// temp.Execute(os.Stdout, TodoInfo{
	// 	Title: repoName,
	// 	// TODO [gen_group_by_filename]: Group todos by filename (again) - let this be a bool flag.
	// 	// For now it's deactivated in order to easily support sorting on fields.
	// 	//FileTodos: make(map[string][]gen.TodoDesc),
	// 	Show:  fieldsShown,
	// 	Todos: drawableTodos,
	// })
}
