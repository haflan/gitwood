package rogit

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

func Deflate(r io.Reader) ([]byte, error) {
	r, err := zlib.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to create the reader: %w", err)
	}
	var buf bytes.Buffer
	_, err = io.Copy(bufio.NewWriter(&buf), r)
	return buf.Bytes(), err
}

func searchPack(idxfile, shasum string) (ObjectType, []byte, error) {
	index, err := SearchPackIDX(idxfile, shasum)
	if err != nil {
		return OBJ_INVALID, nil, err
	}
	if index < 0 {
		return OBJ_INVALID, nil, ErrObjectNotFound
	}
	return ReadPack(strings.Replace(idxfile, ".idx", ".pack", 1), index)
}

func searchAllPacks(gitdir, shasum string) (ObjectType, []byte, error) {
	packdir := path.Join(gitdir, "objects", "pack")
	dir, err := os.ReadDir(packdir)
	if err != nil {
		return OBJ_INVALID, nil, err
	}
	var otype ObjectType
	var object []byte
	for _, entry := range dir {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".idx") {
			otype, object, err = searchPack(packdir+"/"+entry.Name(), shasum)
			if err == nil {
				break
			}
		}
	}
	if object == nil && err == nil {
		return OBJ_INVALID, nil, ErrObjectNotFound
	}
	return otype, object, err
}

func openObject(gitdir, shasum string) (ObjectType, []byte, error) {
	if len(shasum) != 40 {
		return OBJ_INVALID, nil, ErrMalformedShasum
	}
	file, err := os.Open(path.Join(gitdir, "objects", shasum[:2], shasum[2:]))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return OBJ_INVALID, nil, err
		}
		return searchAllPacks(gitdir, shasum)
	}
	defer file.Close()
	c, err := Deflate(file)
	if err != nil {
		return OBJ_INVALID, nil, err
	}
	var otype string
	var i int
	for i = range c {
		if c[i] == 0x20 {
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
