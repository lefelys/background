package background

import (
	"sync"
)

type errBackground struct {
	*group
	err error

	sync.RWMutex
}

// WithError returns new Background with merged children and assigned err to it.
func WithError(err error, children ...Background) Background {
	return withError(err, children...)
}

func withError(err error, children ...Background) *errBackground {
	return &errBackground{
		group: merge(children...),
		err:   err,
	}
}

// Err returns error assigned to errBackground
func (e *errBackground) Err() (err error) {
	e.RLock()
	defer e.RUnlock()

	return e.err
}

func (e *errBackground) DependsOn(children ...Background) Background {
	return withDependency(e, children...)
}
