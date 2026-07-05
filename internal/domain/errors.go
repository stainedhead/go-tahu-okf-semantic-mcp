package domain

import "errors"

// Sentinel domain errors. Callers should use errors.Is for matching.
var (
	ErrNotFound       = errors.New("not found")
	ErrReservedPath   = errors.New("reserved filename")
	ErrMissingType    = errors.New("frontmatter missing required field: type")
	ErrPathEscape     = errors.New("path escapes bundle root")
	ErrInputTooLarge  = errors.New("input exceeds size limit")
	ErrDuplicateAlias = errors.New("bundle alias already registered")
	ErrDuplicatePath  = errors.New("bundle root path already registered")
)
