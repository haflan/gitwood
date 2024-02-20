package gitwood

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// TODO: Stop reading all bytes like this.
// I think it's a trivial change to use a Reader everywhere instead,
// and it's more efficient.
func Decompress(b *bufio.Reader) ([]byte, error) {
	r, err := zlib.NewReader(b)
	if err != nil {
		return nil, fmt.Errorf("failed to create the reader: %w", err)
	}
	defer r.Close()
	var buf bytes.Buffer
	_, err = io.Copy(bufio.NewWriter(&buf), r)
	return buf.Bytes(), err
}

func (r *Repo) searchAllPacks(shasum string) (ObjectType, []byte, error) {
	packdir := path.Join(r.GitDir, "objects", "pack")
	dir, err := os.ReadDir(packdir)
	if err != nil {
		return OBJ_INVALID, nil, err
	}
	var packfile string
	var index int64
	// First find the pack in the index file
	for _, entry := range dir {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".idx") {
			index, err = SearchPackIDX(filepath.Join(packdir, entry.Name()), shasum)
			// Some error occurred, but keep trying other packs, if any
			if err != nil {
				continue
			}
			if index >= 0 {
				packfile = strings.Replace(entry.Name(), ".idx", ".pack", 1)
				break
			}
		}
	}
	if err != nil {
		return OBJ_INVALID, nil, fmt.Errorf("error when searching packs: %w", err)
	}
	if packfile == "" {
		return OBJ_INVALID, nil, ErrObjectNotFound
	}
	return r.OpenAndReadFromPack(filepath.Join(packdir, packfile), uint64(index))
}

// Optimization (here and everywhere): Use Readers instead of reading and returning the entire object

// openObject returns the object type and object data referenced by the given shasum,
// or an error if it doesn't exist.
func (r *Repo) openObject(shasum string) (ObjectType, []byte, error) {
	if len(shasum) != 40 {
		return OBJ_INVALID, nil, ErrMalformedShasum
	}
	file, err := os.Open(path.Join(r.GitDir, "objects", shasum[:2], shasum[2:]))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return OBJ_INVALID, nil, err
		}
		return r.searchAllPacks(shasum)
	}
	defer file.Close()
	c, err := Decompress(bufio.NewReader(file))
	if err != nil {
		return OBJ_INVALID, nil, err
	}
	var otype string
	var i int
	for i = range c {
		if c[i] == CHAR_SPACE {
			i++
			break
		}
		otype += string(c[i])
		if i >= len(c)-1 {
			return OBJ_INVALID, nil, ErrMalformedObject
		}
	}
	// Skip size
	for c[i] != 0 {
		if i >= len(c)-1 {
			return OBJ_INVALID, nil, ErrMalformedObject
		}
		i++
	}
	return ObjectTypeFromString(otype), c[i+1:], nil
}
