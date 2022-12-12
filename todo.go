package main

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var (
	// The timestamp of a commit can be specified by the creator, but will be read from logs otherwise.
	reTimestamp = `((?P<Timestamp>\d{4}-\d{2}-\d{2}(T\d{2}:\d{2}Z)?)\s+)?`
	rePriority  = `(\((?P<Priority>[A-Z])\)\s+)?`
	// Todo ID is *not* optional, because low-effort TODOs should be ignored ;)
	reID    = `(\[(?P<ID>([a-z]|_)+)\]\s*)`
	treBase = fmt.Sprintf(`TODO\s*%v:\s*%v%v(?P<Title>.+)`, reID, rePriority, reTimestamp)
	// Todo Regexps
	treSimple      = regexp.MustCompile(treBase)
	treDoubleSlash = regexp.MustCompile(`\/\/\s*` + treBase)
	treHash        = regexp.MustCompile(`#\s*` + treBase)
	// imp_subseq_line_detection
	extRegexps = map[string]*regexp.Regexp{
		".go":   treDoubleSlash,
		".adoc": treDoubleSlash,
		".py":   treHash,
	}
	extPrefixes = map[string]string{
		".go":   "//",
		".adoc": "//",
		".py":   "#",
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
	Author        string
	Timestamp     time.Time
	TimeAgoString string // formatted timestamp
	// todo.txt fields
	Priority string
}

func (td TodoDesc) String() string {
	return fmt.Sprintf(`%v (line %v): %v`, td.FileName, td.Line, td.Title)
}

type todoExtractor struct {
	filename string
	prefix   string
	rex      *regexp.Regexp
	tii      int
	tti      int
	tpi      int
	tdi      int
}

func getTodoExtractor(filename string) todoExtractor {
	var fileExt string
	var rex *regexp.Regexp
	// Might remove this extension logic altogether in context of imp_subseq_line_detection
	if exti := strings.LastIndex(filename, "."); exti > 0 {
		fileExt = filename[exti:]
		rex = extRegexps[fileExt]
	}
	if rex == nil {
		rex = treSimple
	}
	return todoExtractor{
		filename: filename,
		prefix:   extPrefixes[fileExt],
		rex:      rex,
		tii:      rex.SubexpIndex("ID"),
		tti:      rex.SubexpIndex("Timestamp"),
		tpi:      rex.SubexpIndex("Priority"),
		tdi:      rex.SubexpIndex("Title"),
	}
}

func (tex todoExtractor) Extract(lineNum int, line *git.Line) *TodoDesc {
	if line == nil {
		return nil
	}
	match := tex.rex.FindStringSubmatch(line.Text)
	if len(match) == 0 {
		return nil
	}
	td := &TodoDesc{
		Author:   line.Author,
		FileName: tex.filename,
		Line:     lineNum,
		ID:       match[tex.tii],
		Title:    match[tex.tdi],
		Priority: match[tex.tpi],
	}
	td.Timestamp, _ = time.Parse("2006-01-02T15:04Z", string(match[tex.tti]))
	// NOTE [use_git_blame_when_fixed]: 2022-12-10T21:45 When git.Blame() is fixed, timestamp is available for all Todos
	// if ts.IsZero() {
	// 	ts = line.Date
	// }
	if !td.Timestamp.IsZero() {
		td.TimeAgoString = prettyTime(td.Timestamp)
	}
	return td
}

func (tex todoExtractor) ExtractFull(lineNum int, lines []*git.Line) *TodoDesc {
	if lineNum >= len(lines) {
		return nil
	}
	todo := tex.Extract(lineNum, lines[lineNum])
	lineNum++
	detailLines := []string{}
	// TODO [imp_subseq_line_detection]: Improve detection of subsequent lines.
	// Can't hard code all file extensions, so the detector needs to be smarter.
	// For instance, if a TODO with a certain prefix is found, all subsequent lines
	// with the same prefix should be included (with the prefix trimmed).
	for lineNum < len(lines) {
		lineText := lines[lineNum].Text
		// Empty line should always terminate the todo.
		if strings.TrimSpace(lineText) == "" {
			break
		}
		// Ignore whitespace before the prefix, but not after,
		// because markdown requires two trailing whitespace to force newline IIRC.
		lineText = strings.TrimLeftFunc(lineText, unicode.IsSpace)
		// If the extractor has registered a prefix, any line missing the prefix will terminate the todo.
		if tex.prefix != "" {
			if !strings.HasPrefix(lineText, tex.prefix) {
				break
			}
			lineText = strings.TrimPrefix(lineText, tex.prefix)
		}
		detailLines = append(detailLines, lineText)
		lineNum++
	}
	todo.Details = strings.Join(detailLines, "\n")
	return todo
}

var ErrFileIsBinary = errors.New("the file is binary")

func getLines(c *object.Commit, f *object.File) ([]*git.Line, error) {
	bin, err := f.IsBinary()
	if bin {
		return nil, fmt.Errorf("%v: %w", f.Name, ErrFileIsBinary)
	}
	if err != nil {
		return nil, fmt.Errorf("file to check if %v is binary: %w", f.Name, err)
	}
	lines, err := f.Lines()
	if err != nil {
		return nil, err
	}
	// Workaround needed until go-git fixes its blame bugs
	glines := make([]*git.Line, len(lines))
	for i, line := range lines {
		glines[i] = &git.Line{Text: line}
	}
	// TODO [use_git_blame_when_fixed]: 2022-12-10T20:45Z Use git.Blame instead of the temporary workaround,
	// like this:
	//br, err := git.Blame(c, f.Name)
	//if err != nil {
	//	return nil, err
	//}
	//return br.Lines, nil
	return glines, nil
}

// FindFileTodos finds all the todos in the given git file, assuming it's not a binary type.
func FindFileTodos(c *object.Commit, f *object.File) ([]TodoDesc, error) {
	lines, err := getLines(c, f)
	if errors.Is(err, ErrFileIsBinary) {
		// Binary files should simply be ignored, of course
		return nil, nil
	}
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
func ReadFullTodo(c *object.Commit, f *object.File, lineNum int) (*TodoDesc, error) {
	tex := getTodoExtractor(f.Name)
	lines, err := getLines(c, f)
	if err != nil {
		return nil, err
	}
	todo := tex.ExtractFull(lineNum, lines)
	if todo == nil {
		return nil, fmt.Errorf("could not read todo in %v line %v", f.Name, lineNum)
	}
	return todo, nil
}

func FindCommitTodos(c object.Commit) ([]TodoDesc, error) {
	todos := []TodoDesc{}
	fIter, err := c.Files()
	if err != nil {
		return nil, err
	}
	err = fIter.ForEach(func(f *object.File) error {
		newTodos, err := FindFileTodos(&c, f)
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
// NOTE: This is copied directly from the old skog-gen project - much of it can probably be removed.

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
	return a.Timestamp.Before(b.Timestamp)
}
func todoAllLess(a, b TodoDesc, reverse bool) bool {
	return false
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
}
