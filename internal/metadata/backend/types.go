package backend

import "errors"

var ErrKeyNotFound = errors.New("key not found")

type Entry struct {
	Key *string
	// ModificationCounter can represent a timestamp or a revision number, depending on the backend implementation.
	// Note: Timestamp resolution can vary by S3 backend
	ModificationCounter *int64
}
