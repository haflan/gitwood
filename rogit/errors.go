package rogit

import "errors"

var (
	ErrObjectNotFound  = errors.New("object not found")
	ErrMalformedShasum = errors.New("malformed shasum")
	ErrMalformedObject = errors.New("malformed object")
	ErrMalformedCommit = errors.New("malformed commit")
	ErrNotACommit      = errors.New("not a commit")
)
