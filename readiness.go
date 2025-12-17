package background

import (
	"sync"
)

type readinessBackground struct {
	*group

	ready    chan struct{}
	readyOut chan struct{}

	sync.Mutex
}

// ReadinessTail detaches after readiness Background initialization.
// The tail is supposed to stay in a background job associated with
// created Background as it carries readiness signal.
type ReadinessTail interface {
	// Ok sends a signal that background job is ready.
	// Not calling Ok will block all parents readiness and cause
	// the channel from Background's Ready call to block forever.
	// After the first call, subsequent calls do nothing.
	Ok()
}

func (r *readinessBackground) Ok() {
	r.Lock()
	defer r.Unlock()

	select {
	case <-r.ready:
		// Already ready
	default:
		close(r.ready)
	}
}

func WithReadiness(children ...Background) (Background, ReadinessTail) {
	m := withReadiness(children...)
	return m, m
}

func withReadiness(children ...Background) *readinessBackground {
	s := &readinessBackground{
		group: merge(children...),
		ready: make(chan struct{}),
	}

	return s
}

func (r *readinessBackground) Ready() <-chan struct{} {
	r.Lock()
	defer r.Unlock()

	if r.readyOut != nil {
		// To avoid memory leaks - readyOut channel is created only once
		return r.readyOut
	}

	r.readyOut = make(chan struct{})

	go func() {
		<-r.group.Ready()
		<-r.ready
		close(r.readyOut)
	}()

	return r.readyOut
}

func (r *readinessBackground) DependsOn(children ...Background) Background {
	return withDependency(r, children...)
}
