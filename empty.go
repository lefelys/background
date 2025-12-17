package background

import "context"

type emptyBackground struct{}

// Empty returns new empty Background
func Empty() Background                                    { return emptyBackground{} }
func (e emptyBackground) Err() error                       { return nil }
func (e emptyBackground) Shutdown(_ context.Context) error { return nil }
func (e emptyBackground) Wait()                            {}
func (e emptyBackground) Ready() <-chan struct{}           { return closedchan }
func (e emptyBackground) Value(_ interface{}) interface{}  { return nil }
func (e emptyBackground) DependsOn(children ...Background) Background {
	return withDependency(e, children...)
}
func (e emptyBackground) close()                     {}
func (e emptyBackground) finishSig() <-chan struct{} { return closedchan }
func (e emptyBackground) cause() error               { return nil }
