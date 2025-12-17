package background

import (
	"sync"
)

type waitBackground struct {
	*group
	sync.WaitGroup
}

// WaitTail detaches after waitable background initialization.
// The tail is supposed to stay in a background job associated with
// created Background.
//
// WaitTail uses sync.WaitGroup and shares all its mechanics.
type WaitTail interface {
	// Done calls sync.WaitGroup's Done method
	Done()

	// Add calls sync.WaitGroup's Add method
	Add(i int)
}

// WithWait returns new waitable Background with merged children.
//
// The returned WaitTail is used to increment and decrement Background's WaitGroup counter.
func WithWait(children ...Background) (Background, WaitTail) {
	s := withWait(children...)

	return s, s
}

func withWait(children ...Background) *waitBackground {
	return &waitBackground{
		group: merge(children...),
	}
}

// Wait blocks until Backgrounds's and Backgrounds's children counters are zero.
func (w *waitBackground) Wait() {
	w.WaitGroup.Wait()
	w.group.Wait()
}

func (w *waitBackground) DependsOn(children ...Background) Background {
	return withDependency(w, children...)
}
