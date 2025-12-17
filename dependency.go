package background

import (
	"context"
	"sync"
)

type dependBackground struct {
	children *group
	parent   Background

	finished chan struct{}
	ready    chan struct{}

	sync.RWMutex
}

// withDependency returns new Background with merged parent and children
// with parent's dependency set on children.
func withDependency(parent Background, children ...Background) *dependBackground {
	return &dependBackground{
		children: merge(children...),
		parent:   parent,
		finished: make(chan struct{}),
	}
}

func (d *dependBackground) Shutdown(ctx context.Context) error {
	return shutdown(ctx, d)
}

func (d *dependBackground) close() {
	d.children.close()
	<-d.children.finishSig()

	d.parent.close()
	<-d.parent.finishSig()
	d.Done()
}

func (d *dependBackground) Done() {
	d.Lock()
	defer d.Unlock()
	select {
	case <-d.finished:
		// Already closed
	default:
		close(d.finished)
	}
}

func (d *dependBackground) Wait() {
	d.children.Wait()
	d.parent.Wait()
}

func (d *dependBackground) Ready() <-chan struct{} {
	d.Lock()
	defer d.Unlock()

	if d.ready != nil {
		// To avoid memory leaks - ready channel is created only once
		return d.ready
	}

	d.ready = make(chan struct{})

	go func() {
		<-d.children.Ready()
		<-d.parent.Ready()
		close(d.ready)
	}()

	return d.ready
}

func (d *dependBackground) Err() (err error) {
	if err = d.parent.Err(); err != nil {
		return err
	}

	for _, backgrounds := range d.children.backgrounds {
		if err = backgrounds.Err(); err != nil {
			return err
		}
	}

	return
}

func (d *dependBackground) Value(key interface{}) (value interface{}) {
	if value = d.parent.Value(key); value != nil {
		return value
	}

	for _, backgrounds := range d.children.backgrounds {
		if value = backgrounds.Value(key); value != nil {
			return value
		}
	}

	return
}

func (d *dependBackground) DependsOn(children ...Background) Background {
	return d.dependsOn(children...)
}

func (d *dependBackground) dependsOn(children ...Background) *dependBackground {
	return withDependency(d, children...)
}

func (d *dependBackground) finishSig() <-chan struct{} {
	return d.finished
}

func (d *dependBackground) cause() error {
	err := d.children.cause()
	if err != nil {
		return err
	}

	err = d.parent.cause()
	if err != nil {
		return err
	}

	return nil
}
