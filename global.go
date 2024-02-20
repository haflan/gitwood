package gitwood

import "errors"

const (
	CHAR_SPACE = 0x20
	NULL_HASH  = "0000000000000000000000000000000000000000"
)

var (
	ErrObjectNotFound  = errors.New("object not found")
	ErrMalformedShasum = errors.New("malformed shasum")
	ErrMalformedObject = errors.New("malformed object")
	ErrMalformedCommit = errors.New("malformed commit")
	ErrNotATree        = errors.New("object is not a tree")
	ErrNotACommit      = errors.New("not a commit")
)
