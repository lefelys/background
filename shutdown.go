package background

import (
	"context"
	"sync"
)

type shutdownBackground struct {
	*group

	end  chan struct{}
	done chan struct{}

	sync.Mutex
}

// ShutdownTail detaches after shutdownable Background initialization.
// The tail is supposed to stay in a background job associated with
// created Background as it carries shutdown and finish signals.
type ShutdownTail interface {
	// End returns a channel that's closed when work done on behalf
	// of tail's Background should be shut down.
	// Successive calls to End return the same value.
	End() <-chan struct{}

	// Done sends a signal that a shutdown is complete.
	// Not calling Done will block all parents closing and cause
	// the Background's Shutdown call to return ErrTimeout or block forever.
	// After the first call, subsequent calls do nothing.
	Done()
}

func (s *shutdownBackground) End() (c <-chan struct{}) {
	return s.end
}

func (s *shutdownBackground) Done() {
	s.Lock()
	defer s.Unlock()

	select {
	case <-s.done:
		// Already closed
	default:
		close(s.done)
	}
}

// closer is used for graceful shutdown.
type closer interface {
	// close sends close signal to the Background and blocks until the closing
	// is complete.
	close()

	// finishSig returns a channel that's closed when the closing
	// is complete.
	finishSig() <-chan struct{}

	// cause walks down the tree of Backgrounds to find the first full path
	// of unclosed children to accumulate annotations. There is a
	// chance that the closing will complete during that check -
	// in this case it is considered as fully completed and returns nil.
	cause() error
}

// shutdown is a function for shutting down Backgrounds that implements
// closer interface
func shutdown(ctx context.Context, c closer) error {
	go c.close()

	select {
	case <-c.finishSig():
		return nil
	case <-ctx.Done():
		return c.cause()
	}
}

// WithShutdown returns a new shutdownable Background that depends on children.
//
// The returned ShutdownTail's End channel is closed when Background's Shutdown
// method is called or by its parent during graceful shutdown.
//
// The ShutdownTail's Done call sends a signal that the shutdown is complete,
// which causes Background's Shutdown method to return nil, or allow its parent
// to shut down itself during graceful shutdown.
func WithShutdown(children ...Background) (Background, ShutdownTail) {
	m := withShutdown(children...)
	return m, m
}

func withShutdown(children ...Background) *shutdownBackground {
	s := &shutdownBackground{
		group: merge(children...),
		done:  make(chan struct{}),
		end:   make(chan struct{}),
	}

	return s
}

// Shutdown gracefully shuts down the shutdown Background.
// Shutdown shuts down its children first, wait until all of them
// are successfully shut down and then shuts down itself.
func (s *shutdownBackground) Shutdown(ctx context.Context) error {
	return shutdown(ctx, s)
}

func (s *shutdownBackground) close() {
	go s.group.close()
	<-s.group.finishSig()

	s.Lock()
	defer s.Unlock()

	select {
	case <-s.end:
		return // Already closed
	default:
		close(s.end)
	}
}

func (s *shutdownBackground) finishSig() <-chan struct{} {
	return s.done
}

func (s *shutdownBackground) DependsOn(children ...Background) Background {
	return withDependency(s, children...)
}

func (s *shutdownBackground) cause() error {
	if err := s.group.cause(); err != nil {
		return err
	}

	select {
	case <-s.done:
		return nil
	default:
		return ErrTimeout
	}
}
