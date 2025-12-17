package background

import "fmt"

// ErrTail detaches after error group Background initialization.
// The tail is supposed to stay in a background job associated with
// created Background and used to assign error to it.
type ErrTail interface {
	// Error assigns err to associated background.
	// If the background already has an error - does nothing.
	Error(err error)

	// Errorf formats according to a format specifier and assigns
	// the string to associated background as a value that satisfies error.
	// If the background already has an error - does nothing.
	Errorf(format string, a ...interface{})
}

type errGroupBackground struct {
	*errBackground
}

// WithErrorGroup returns new background with merged children that can
// store an error.
//
// The returned ErrTail is used to assign error to the background.
func WithErrorGroup(children ...Background) (Background, ErrTail) {
	b := withErrorGroup(children...)
	return b, b
}

func withErrorGroup(children ...Background) *errGroupBackground {
	return &errGroupBackground{errBackground: withError(nil, children...)}
}

// Error assigns err to the Background.
//
// If the Background already has an error - does nothing.
func (e *errGroupBackground) Error(err error) {
	if err != nil {
		e.Lock()
		if e.err == nil {
			e.err = err
		}
		e.Unlock()
	}
}

// Errorf formats according to a format specifier and assigns
// the string to the Background as a value that satisfies error.
//
// If the Background already has an error - does nothing.
//
// Uses fmt.Errorf thus supports error wrapping with %w verb.
func (e *errGroupBackground) Errorf(format string, a ...interface{}) {
	e.Error(fmt.Errorf(format, a...))
}

func (e *errGroupBackground) DependsOn(children ...Background) Background {
	return withDependency(e, children...)
}
