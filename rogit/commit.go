package rogit

import (
	"fmt"
	"strings"
)

type Commit struct {
	ShaSum    string
	Tree      string
	Parent    string
	Author    string
	Committer string
	Message   string
}

func (c Commit) String() string {
	return fmt.Sprintf(
		"commit %v\nAuthor: %v\nTree: %v\n\n%v",
		c.ShaSum, c.Author, c.Tree, c.Message,
	)
}

func ParseCommit(shasum, commitDef string) (*Commit, error) {
	var i int
	lines := strings.Split(commitDef, "\n")
	commit := Commit{ShaSum: shasum}
	for i = range lines {
		line := lines[i]
		if line == "" {
			i++
			break
		}
		fs := strings.Index(lines[i], " ")
		switch line[:fs] {
		case "tree":
			commit.Tree = line[fs+1:]
		case "parent":
			commit.Parent = line[fs+1:]
		case "author":
			commit.Author = line[fs+1:]
		case "committer":
			commit.Committer = line[fs+1:]
		}
	}
	commit.Message = strings.Join(lines[i:], "\n")
	return &commit, nil
}
