package gitwood

import (
	"fmt"
	"strings"
)

type Commit struct {
	ShaSum    string
	Tree      string
	Parents   []string
	Author    string
	Committer string
	Message   string
	repo      Repo
}

func (c Commit) String() string {
	var parents string
	for i, p := range c.Parents {
		parents += fmt.Sprintf("Parent[%d]: %s\n", i, p)
	}
	out := fmt.Sprintf(
		"commit %v\nAuthor: %v\nTree: %v\n%v\n%v\n",
		c.ShaSum, c.Author, c.Tree, parents, c.Message,
	)
	return out
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
			commit.Parents = append(commit.Parents, line[fs+1:])
		case "author":
			commit.Author = line[fs+1:]
		case "committer":
			commit.Committer = line[fs+1:]
		}
	}
	if commit.Tree == "" {
		return nil, fmt.Errorf("no tree found in commit %v", shasum)
	}
	// Check for author and committer too? Not sure what's mandatory.
	commit.Message = strings.Join(lines[i:], "\n")
	return &commit, nil
}

func (r Repo) Commit(sha string) (*Commit, error) {
	otype, o, err := r.Object(sha)
	if err != nil {
		return nil, err
	}
	if otype != OBJ_COMMIT {
		return nil, ErrNotACommit
	}
	commit, err := ParseCommit(sha, string(o))
	if err != nil {
		return nil, err
	}
	commit.repo = r
	return commit, nil
}

func (c Commit) WalkToPath(path string, tw TreeWalker) (ObjectType, []byte, error) {
	tree, err := c.repo.Tree(c.Tree)
	if err != nil {
		return OBJ_INVALID, nil, fmt.Errorf("failed to open tree %v: %w", c.Tree, err)
	}
	return tree.WalkToPath(path, tw)
}
