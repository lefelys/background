// The background package provides simple background jobs management primitives for go applications.
//
// The package defines the Background type, which carries errors, wait groups,
// shutdown signals and other values from application's background jobs.
//
// The Background type is aggregative - it contains multiple backgrounds in tree form,
// allowing setting dependencies for graceful shutdown between them and merging
// multiple independent backgrounds.
//
// To aggregate the application's background jobs, functions that initialize
// them create suitable Background and propagate it up in the calls stack to the
// layer where it will be handled, optionally merging it with other backgrounds,
// setting dependencies between them and annotating along the way.
//
// Programs that use Background should follow these rules to keep interfaces
// consistent:
//
// 1. All functions that initialize application-scoped background jobs should
// return Background as its last return value.
//
// There might be special cases, when returning Background as the last return value is
// not possible, for example - when using dependency injection packages. To handle
// this case, embed Background into dependency's return value:
//
//	type Server struct {
//		background.Background
//
//		server *http.Server
//	}
//
//	type Updater interface {
//		background.Background
//
//		Update() error
//	}
//
//	func NewApp(server *Server, updater Updater) background.Background {
//		 bg := server.DependsOn(updater)
//
//		 /*...*/
//	}
//
// 2. If an error can occur during initialization it is still should be returned
// as Background using function WithError.
//
// 3. Never return nil Background - return Empty() instead, or do not return Background
// at all if it is not needed.
//
// 4. Every background job should be shutdownable and/or waitable.
package background

import (
	"context"
	"errors"
)

// Background carries errors, wait groups, shutdown signals and other values
// from application's background jobs in tree form.
//
// Background is not reusable.
//
// Background's methods may be called by multiple goroutines simultaneously.
type Background interface {
	// Err returns the first encountered error in this Background.
	// While error is propagated from bottom to top, it is being annotated
	// by annotation Backgrounds in a chain. Annotation uses introduced in
	// go 1.13 errors wrapping.
	//
	// Successive calls to Err may not return the same value, but it will
	// never return nil after the first error occurred.
	Err() error

	// Wait blocks until all counters of WaitGroups in this Background are zero.
	// It uses sync.Waitgroup under the hood and shares all its mechanics.
	Wait()

	// Shutdown gracefully shuts down this Background.
	// Ths shutdown occurs from bottom to top: parents shut down their
	// children, wait until all of them are successfully shut down and
	// then shut down themselves.
	//
	// If ctx expires before the shutdown is complete, Shutdown tries
	// to find the first full path of unclosed children to accumulate
	// annotations and returns ErrTimeout wrapped in them.
	// There is a chance that the shutdown will complete during that check -
	// in this case, it is considered as fully completed and returns nil.
	Shutdown(ctx context.Context) error

	// Ready returns a channel that signals that all Backgrounds in tree are
	// ready. If there is no readiness Backgrounds in the tree - Background is considered
	// as ready by default.
	//
	// If some readiness Background in the tree didn't send Ok signal -
	// returned channel blocks forever. It is caller's responsibility to
	// handle possible block.
	Ready() <-chan struct{}

	// Value returns the first found value in this Background for key,
	// or nil if no value is associated with key. The tree is searched
	// from top to bottom and from left to right.
	//
	// It is possible to have multiple values associated with the same key,
	// but Value call will always return the topmost and the leftmost.
	//
	// Use Background values only for data that represents custom Backgrounds, not
	// for returning optional values from functions.
	//
	// Other rules for working with Value is the same as in the standard
	// package context:
	//
	// 1. Functions that wish to store values in Background typically allocate
	// a key in a global variable then use that key as the argument to
	// background.WithValue and Background.Value.
	// 2. A key can be any type that supports equality and can not be nil.
	// 3. Packages should define keys as an unexported type to avoid
	// collisions.
	// 4. Packages that define a Background key should provide type-safe accessors
	// for the values stored using that key (see examples).
	Value(key interface{}) (value interface{})

	// DependsOn creates a new Background from the original and children.
	// The new Background ensures that during shutdown it will shut down children
	// first, wait until all of them are successfully shut down and then shut
	// down the original Background.
	DependsOn(children ...Background) Background

	// closer is a private inteface used for graceful shutdown. It is
	// necessary to have it in exported interface for cases of embedding
	// Background into another struct.
	closer
}

var (
	// ErrTimeout is the error returned by Background.Shudown when shutdown's
	// timeout is expired
	ErrTimeout = errors.New("timeout expired")

	// closedchan is a reusable closed channel.
	closedchan = make(chan struct{})
)

func init() {
	close(closedchan)
}
