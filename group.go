package background

import (
	"context"
	"sync"
)

type group struct {
	backgrounds []Background
	toClose     map[int]struct{}

	done, finished chan struct{}
	ready          chan struct{}

	sync.RWMutex
}

// Merge returns new Background with merged children.
func Merge(bgs ...Background) Background {
	return merge(bgs...)
}

func merge(bgs ...Background) *group {
	if len(bgs) == 0 {
		return &group{
			done:     closedchan,
			finished: closedchan,
		}
	}

	var (
		ss       = make([]Background, 0, len(bgs))
		done     = make(chan struct{})
		finished = make(chan struct{})
		toClose  = make(map[int]struct{})
	)

	for i, s := range bgs {
		if s == nil {
			continue
		}

		ss = append(ss, s)

		select {
		case <-s.finishSig():
			// already closed
		default:
			toClose[i] = struct{}{}

			addToCloseStream(done, s)
		}
	}

	return &group{
		backgrounds: ss,
		toClose:     toClose,
		done:        done,
		finished:    finished,
	}
}

func addToCloseStream(done <-chan struct{}, c Background) {
	go func() {
		<-done
		c.close()
	}()
}

func (g *group) Shutdown(ctx context.Context) error {
	return shutdown(ctx, g)
}

func (g *group) finishSig() <-chan struct{} {
	return g.finished
}

func (g *group) Wait() {
	for _, m := range g.backgrounds {
		m.Wait()
	}
}

func (g *group) Ready() <-chan struct{} {
	g.Lock()
	defer g.Unlock()

	if g.ready != nil {
		// To avoid memory leaks - ready channel is created only once
		return g.ready
	}

	g.ready = make(chan struct{})

	go func() {
		for _, m := range g.backgrounds {
			<-m.Ready()
		}

		close(g.ready)
	}()

	return g.ready
}

func (g *group) close() {
	g.Lock()
	select {
	case <-g.done:
		g.Unlock()
		return // already closed
	default:
		close(g.done)
	}
	g.Unlock()

	for i := range g.toClose {
		<-g.backgrounds[i].finishSig()
		g.Lock()
		delete(g.toClose, i)
		g.Unlock()
	}

	close(g.finished)
}

func (g *group) Err() error {
	for _, bg := range g.backgrounds {
		if err := bg.Err(); err != nil {
			return err
		}
	}

	return nil
}

func (g *group) Value(key interface{}) (value interface{}) {
	for _, bg := range g.backgrounds {
		if value = bg.Value(key); value != nil {
			return value
		}
	}

	return nil
}

func (g *group) DependsOn(children ...Background) Background {
	return withDependency(g, children...)
}

func (g *group) cause() error {
	g.RLock()
	defer g.RUnlock()

	for _, bg := range g.backgrounds {
		if err := bg.cause(); err != nil {
			return err
		}
	}

	return nil
}
