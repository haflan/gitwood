package rogit

import (
	"encoding/hex"
	"fmt"
)

// Tree format:
// tree [content size]\0[Entries having references to other trees and blobs].
// [mode] [file/folder name]\0[SHA-1 of referencing blob or tree]

type TreeEntry struct {
	Mode   string
	Path   string
	ShaSum string
}

func (te TreeEntry) String() string {
	return fmt.Sprintf("[%s] [%s] [%s]", te.Mode, te.Path, te.ShaSum)
}

func ExtractTreeEntries(tree []byte) []TreeEntry {
	var i int
	entries := []TreeEntry{}
	for i < len(tree) {
		ne := TreeEntry{}
		for tree[i] != 0x20 {
			ne.Mode += string(tree[i])
			i++
		}
		i++
		for i < len(tree) && tree[i] != 0 {
			ne.Path += string(tree[i])
			i++
		}
		i++
		ne.ShaSum = hex.EncodeToString(tree[i : i+20])
		entries = append(entries, ne)
		i += 20
	}
	return entries
}
