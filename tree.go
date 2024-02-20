package gitwood

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

// Tree format:
// tree [content size]\0[Entries having references to other trees and blobs].
// [mode] [file/folder name]\0[SHA-1 of referencing blob or tree]

type Tree struct {
	shaSum     string
	objectData []byte
	repo       Repo
}

type TreeEntry struct {
	Mode   string
	name   string
	ShaSum string
}

func (te TreeEntry) String() string {
	name := te.name
	if te.IsDir() {
		name += "/"
	}
	return fmt.Sprintf("(%s) [%6s] %s", te.ShaSum[:9], te.Mode, name)
}

func (te TreeEntry) IsDir() bool {
	return te.Mode == "40000"
}

func (te TreeEntry) Name() string {
	return te.name
}

func ExtractTreeEntries(tree []byte) []TreeEntry {
	var i int
	entries := []TreeEntry{}
	for i < len(tree) {
		ne := TreeEntry{}
		for tree[i] != CHAR_SPACE {
			ne.Mode += string(tree[i])
			i++
		}
		i++
		for i < len(tree) && tree[i] != 0 {
			ne.name += string(tree[i])
			i++
		}
		i++
		ne.ShaSum = hex.EncodeToString(tree[i : i+20])
		entries = append(entries, ne)
		i += 20
	}
	return entries
}

func (r Repo) Tree(shasum string) (*Tree, error) {
	otype, o, err := r.Object(shasum)
	if err != nil {
		return nil, err
	}
	if otype != OBJ_TREE {
		return nil, ErrNotATree
	}
	return &Tree{shaSum: shasum, objectData: o, repo: r}, nil
}

// WalkToPath walks the tree until it finds path (if it exists),
// calling the given TreeWalker function for each entry it finds on the way.
func (t Tree) WalkToPath(path string, w TreeWalker) (otype ObjectType, o []byte, err error) {
	if path == "." || path == "" {
		return OBJ_TREE, t.objectData, nil
	}
	// If no TreeWalker is given, use a dummy one.
	if w == nil {
		w = func(path, sum string) error { return nil }
	}

	o = t.objectData
	// Traverse the tree until the leaf node
	nodes := strings.Split(strings.Trim(path, "/"), "/")
	dirs, filename := nodes[:len(nodes)-1], nodes[len(nodes)-1]

checkTrees:
	for i, name := range dirs {
		for _, e := range ExtractTreeEntries(o) {
			err = w(filepath.Join(append(nodes[:i+1], e.name)...), e.ShaSum)
			if err != nil {
				return
			}
			if e.name != name {
				continue
			}
			otype, o, err = t.repo.Object(e.ShaSum)
			if err != nil {
				return
			}
			if otype != OBJ_TREE {
				err = ErrNotATree
				return
			}
			continue checkTrees
		}
		return OBJ_INVALID, nil, ErrObjectNotFound
	}
	for _, e := range ExtractTreeEntries(o) {
		err = w(filepath.Join(append(dirs, e.name)...), e.ShaSum)
		if err != nil {
			return
		}
		if e.name == filename {
			return t.repo.Object(e.ShaSum)
		}
	}
	return OBJ_INVALID, nil, ErrObjectNotFound
}

type TreeWalker func(path, sum string) error
