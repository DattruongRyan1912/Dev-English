package platform

import "errors"

// Sentinel errors let callers distinguish manifest and dependency failures
// with errors.Is while the returned error still includes useful context.
var (
	ErrInvalidDescriptor = errors.New("invalid module descriptor")
	ErrInvalidManifest   = errors.New("invalid module manifest")
	ErrDuplicateModule   = errors.New("duplicate module")
	ErrUnknownModule     = errors.New("unknown module")
	ErrMissingDependency = errors.New("missing module dependency")
	ErrDependencyCycle   = errors.New("module dependency cycle")
	ErrNilRegistry       = errors.New("nil module registry")
)
