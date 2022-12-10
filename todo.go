package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var (
	reDate      = `(?P<Timestamp>\d{4}-\d{2}-\d{2}(T\d{2}:\d{2})?\s+)?`
	rePriority  = `(?P<Priority>\([A-Z]\)\s+)?`
	reReference = `(\[(?P<ID>([a-z]|_)+)\]\s*)?`
	treBase     = fmt.Sprintf(`TODO\s*%v:\s*%v%v(?P<Title>.+)`, reReference, rePriority, reDate)
	// Todo Regexps
	treSimple      = regexp.MustCompile(treBase)
	treDoubleSlash = regexp.MustCompile(`\/\/\s*` + treBase)
	treHash        = regexp.MustCompile(`#\s*` + treBase)
	extRegexps     = map[string]*regexp.Regexp{
		".go":   treDoubleSlash,
		".adoc": treDoubleSlash,
		".py":   treHash,
	}
)

type TodoDesc struct {
	// FileName path relative to some root
	FileName string
	Line     int
	ID       string
	Title    string
	Details  string
	// *Blame* can be used for Author and Timestamp:
	// https://github.com/go-git/go-git/blob/master/blame.go#L105
	Author    string
	Timestamp string
	// todo.txt fields
	Priority string
}

func (td TodoDesc) String() string {
	return fmt.Sprintf(`%v (line %v): %v`, td.FileName, td.Line, td.Title)
}

type todoExtractor struct {
	filename string
	rex      *regexp.Regexp
	tii      int
	tti      int
	tpi      int
	tdi      int
}

func (tex todoExtractor) Extract(lineNum int, line string) *TodoDesc {
	match := tex.rex.FindStringSubmatch(line)
	if len(match) == 0 {
		return nil
	}
	return &TodoDesc{
		FileName:  tex.filename,
		Line:      lineNum,
		ID:        string(match[tex.tii]),
		Title:     string(match[tex.tdi]),
		Priority:  string(match[tex.tpi]),
		Timestamp: string(match[tex.tti]),
	}
}

func (tex todoExtractor) ExtractFull(lineNum int, lines []string) *TodoDesc {
	todo := tex.Extract(lineNum, lines[lineNum])
	lineNum++
	// TODO [todo_extractor_full]: Extract details from subsequent Todo lines
	return todo
}

func getTodoExtractor(filename string) todoExtractor {
	// TODO [imp_subseq_line_detection]: Improve detection of subsequent lines.
	// Can't hard code all file extensions, so the detector needs to be smarter.
	// This is most important for the Todo body (which should be a new function):
	// For instance, if a TODO with a certain prefix is found, all subsequent lines
	// with the same prefix should be included (with the prefix trimmed).
	exti := strings.LastIndex(filename, ".")
	var rex *regexp.Regexp
	if exti > 0 {
		rex = extRegexps[filename[exti:]]
	}
	if rex == nil {
		rex = treSimple
	}
	return todoExtractor{
		filename: filename,
		rex:      rex,
		tii:      rex.SubexpIndex("ID"),
		tti:      rex.SubexpIndex("Timestamp"),
		tpi:      rex.SubexpIndex("Priority"),
		tdi:      rex.SubexpIndex("Title"),
	}
}

func getLines(f *object.File) ([]string, error) {
	bin, err := f.IsBinary()
	if bin {
		return nil, fmt.Errorf("file %v is binary", f.Name)
	}
	if err != nil {
		return nil, fmt.Errorf("file to check if %v is binary: %w", f.Name, err)
	}
	return f.Lines()
}

// FindFileTodos finds all the todos in the given git file, assuming it's not a binary type.
func FindFileTodos(f *object.File) ([]TodoDesc, error) {
	lines, err := getLines(f)
	if err != nil {
		return nil, err
	}
	todos := []TodoDesc{}
	tex := getTodoExtractor(f.Name)
	for lnum, line := range lines {
		todo := tex.Extract(lnum, line)
		if todo != nil {
			todos = append(todos, *todo)
		}
	}
	return todos, nil
}

// ReadFullTodo tries to read the full todo from the given git file and line number,
// including details from subsequent lines.
func ReadFullTodo(f *object.File, lineNum int) (*TodoDesc, error) {
	tex := getTodoExtractor(f.Name)
	lines, err := getLines(f)
	if err != nil {
		return nil, err
	}
	todo := tex.ExtractFull(lineNum, lines)
	if todo == nil {
		return nil, fmt.Errorf("could not read todo in %v line %v", f.Name, lineNum)
	}
	return todo, nil
}

func FindCommitTodos(repo *git.Repository, hash plumbing.Hash) ([]TodoDesc, error) {
	cob, err := repo.CommitObject(hash)
	if err != nil {
		return nil, err
	}
	var todos []TodoDesc
	fIter, err := cob.Files()
	if err != nil {
		return nil, err
	}
	err = fIter.ForEach(func(f *object.File) error {
		newTodos, err := FindFileTodos(f)
		if err != nil {
			return err
		}
		todos = append(todos, newTodos...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return todos, nil
}

// Sorting and filtering

type TodoFieldsShown struct {
	Description bool
	FileName    bool
	Line        bool
	Priority    bool
	Reference   bool
	Timestamp   bool
}

func MakeFieldsShown(toShowCSV string) TodoFieldsShown {
	if toShowCSV == "" {
		return TodoFieldsShown{true, true, true, true, true, true}
	}
	shown := TodoFieldsShown{}
	for _, show := range strings.Split(toShowCSV, ",") {
		switch show {
		case "desc":
			shown.Description = true
		case "file":
			shown.FileName = true
		case "line":
			shown.Line = true
		case "pri":
			shown.Priority = true
		case "id":
			shown.Reference = true
		case "ts":
			shown.Timestamp = true
		}
	}
	return shown
}

func MakeFieldsShownFromHidden(toHideCSV string) TodoFieldsShown {
	shown := TodoFieldsShown{true, true, true, true, true, true}
	if toHideCSV == "" {
		return shown
	}
	for _, hide := range strings.Split(toHideCSV, ",") {
		switch hide {
		case "desc":
			shown.Description = false
		case "file":
			shown.FileName = false
		case "line":
			shown.Line = false
		case "pri":
			shown.Priority = false
		case "ref":
			shown.Reference = false
		case "ts":
			shown.Timestamp = false
		}
	}
	return shown
}

// Sort string empty last
func ssel(a, b string, reverse bool) (ret bool) {
	if b == "" {
		ret = a != ""
	} else if a == "" {
		ret = b == ""
	} else {
		ret = a < b
	}
	if reverse {
		ret = !ret
	}
	return
}

type fieldLessFunction = func(TodoDesc, TodoDesc, bool) bool

func todoDescLess(a, b TodoDesc, reverse bool) bool {
	return ssel(a.Title, b.Title, reverse)
}
func todoFileLess(a, b TodoDesc, reverse bool) bool {
	return ssel(a.FileName, b.FileName, reverse)
}
func todoPriLess(a, b TodoDesc, reverse bool) bool {
	return ssel(a.Priority, b.Priority, reverse)
}
func todoRefLess(a, b TodoDesc, reverse bool) bool {
	return ssel(a.ID, b.ID, reverse)
}
func todoTsLess(a, b TodoDesc, reverse bool) bool {
	return a.Timestamp < b.Timestamp
}
func todoAllLess(a, b TodoDesc, reverse bool) bool {
	return false
}
func todoLineLess(a, b TodoDesc, reverse bool) bool {
	less := a.Line < b.Line
	if reverse {
		return !less
	}
	return less
}

func Sort(todos []TodoDesc, fields []string) {
	if len(fields) == 0 || (len(fields) == 1 && fields[0] == "") {
		return
	}
	for _, f := range fields {
		if f == "" {
			continue
		}
		var reverse bool
		if f[0] == '-' {
			reverse = true
			f = f[1:]
		}
		var sortFunc fieldLessFunction
		switch f {
		case "desc":
			sortFunc = todoDescLess
		case "file":
			sortFunc = todoFileLess
		case "pri":
			sortFunc = todoPriLess
		case "ref":
			sortFunc = todoRefLess
		case "ts":
			sortFunc = todoTsLess
		default:
			sortFunc = todoAllLess
		}
		sort.SliceStable(todos, func(i, j int) bool {
			return sortFunc(todos[i], todos[j], reverse)
		})
	}
	return
}

func Filter(todos []TodoDesc, fields []string) []TodoDesc {
	if len(fields) == 0 || (len(fields) == 1 && fields[0] == "") {
		return todos
	}
	fReq := struct {
		Reference   bool
		Description bool
		Priority    bool
		Timestamp   bool
	}{}
	for _, f := range fields {
		switch f {
		case "desc":
			fReq.Description = true
		case "pri":
			fReq.Priority = true
		case "ref":
			fReq.Reference = true
		case "ts":
			fReq.Timestamp = true
		}
	}
	n := 0
	for _, todo := range todos {
		if fReq.Description && todo.Title == "" {
			continue
		}
		if fReq.Priority && todo.Priority == "" {
			continue
		}
		if fReq.Reference && todo.ID == "" {
			continue
		}
		if fReq.Timestamp && todo.Timestamp == "" {
			continue
		}
		todos[n] = todo
		n++
	}
	return todos[:n]
}
