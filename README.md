# background

The package provides a simple mechanism for managing application's background jobs, allowing easy to manage **graceful shutdown**, **waiting** for completion, and **error propagation** and **custom propagators** creation.

## Features

- **Graceful Shutdown:** easily define shutdown order for your application without juggling with channels and goroutines
- **Readiness:** wait until all background jobs report that they are ready
- **Waiting:** wait for completion for all waitable background jobs
- **Annotating**: quickly find the cause of errors or frozen shutdowns
- **Error Group**: errors propagation across the tree of backgrounds
- **Custom propagators**

## Prerequisites

Package background defines the `Background` type, which carries errors, wait groups, shutdown signals and other values from application's background jobs.

The `Background` type is aggregative - it contains multiple Backgrounds in tree form, allowing graceful shutdown order to be set between them and multiple independent Backgrounds to be merged.

To aggregate the application's background jobs, functions that initialize them create suitable `Background` and propagate it **up** in the calls stack to the layer where it will be handled, optionally merging it with other Backgrounds, setting shutdown order between them and annotating.

Background has some mechanic similarities with context package.

In the case of context, the `Context` type carries **request-scoped** deadlines, cancelation signals, and other values across API boundaries and between processes.

A common example of `Context` usage – application receives a user request, creates context and makes a request to external API with it:

```
1. o-------------------------------------->  user request
2.          o------------------->            API request
```

By propagating cancelable `Context`, calling the `CancelFunc` during user request cancels the parent and all its children simultaneously:

```
1. o------------[cancel]-x                   user request
2.          o-------------x                  API request
```

`Context` is short-lived and is not supposed to be stored or reused.

`Background`, on the other hand, is **application-scoped**. It provides a type similar to `Context` for controlling long-living background jobs. Unlike Context, `Background` propagates bottom-top during the app initialization phase and it is ok to store it, but not reuse.

Simple example – application initializes consumer and processor to work in background:

```
1. o-------------------------------------->  consumer
2.          o----------------------------->  processor
```

If we want to shut down the application, `Context` canceling semantics is not applicable - a simultaneous shutdown of consumer and processor can cause a data loss. Background package will handle this case gracefully: it will shut down consumer, wait until it signals Ok, and shut down processor after:

```
1. o-----------[close]~~~~[ok]-x             consumer
2.          o-------------[close]~~~~[ok]-x  processor
```

## Getting started

### Requirements

Go 1.13+

### Installing

```
go get -u github.com/lefelys/background
```

### Usage

#### Creation

Create a new `Background` using one of the package functions: `WithShutdown`, `WithWait`, `WithReadiness`, `WithError`, `WithErrorGroup` or `WithValue`:

```go
bg := background.WithError(errors.New("error"))
```

#### Tails

Functions `WithShutdown`, `WithWait`, `WithReadiness`, and `WithErrorGroup` return a detached tail in addition to a new `Background`. The tail must be used for signaling in the background job associated with the `Background`. In the case of `WithShutdown`, it detaches a `ShutdownTail` interface with two methods:

- `End() <-chan struct{}` - returns a channel that's closed when work done on behalf of tail's `Background` should be shut down.
- `Done()` - sends a signal that the shutdown is complete.

```go
bg, tail := background.WithShutdown()

go func() {
	for {
		select {
		case <-tail.End():
			fmt.Println("shutdown job")
			tail.Done()
			return
		default:
			/*...*/
		}
	}
}()
```

`WithReadiness` returns a `ReadinessTail`. Call its `Ok()` method when the associated job is ready. The channel returned by `Background.Ready()` closes after every readiness Background in the tree has reported that it is ready.

#### Propagation

Created `Background` should be propagated up in the calls stack, where it will be handled:

```go
func StartJob() background.Background {
	bg, tail := background.WithShutdown()

	go func() {
		/*...*/
	}()

	return bg
}

func main() {
	jobBg := StartJob()

	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	<-shutdownSig

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := jobBg.Shutdown(ctx)
	if err != nil {
		log.Fatal(err)
	}
}
```

#### Merging and annotating

To merge multiple Backgrounds use function `Merge`. Backgrounds can also be merged using function `WithAnnotation` - it will help to find the cause of errors or frozen shutdowns:

```go
func StartJobs() background.Background {
	bg1 := startJob1()
	bg2 := startJob2()

	return background.WithAnnotation("job1 and job2", bg1, bg2)
}

func main() {
	bg := StartJobs()

	/*...*/

	err := bg.Shutdown(ctx)
	if err != nil {
		log.Fatal(err) // "job1 and job2: timeout expired"
	}
}
```

#### Shutdown order

Shutdown order prevents data loss between related background jobs. A Background configured to shut down after other Backgrounds will shut them down first, wait until all of them are successfully shut down and then shut down itself.
There are 2 ways to configure shutdown order between Backgrounds:

1. By passing the Backgrounds that should shut down first to a Background initializer:

```go
bg1 := startJob1()
bg2 := startJob2()

// bg3 will shut down after bg1 and bg2
bg3, tail := background.WithShutdown(bg1, bg2)
```

2. By calling an existing Background's `ShutdownAfter` method, which returns a new `Background` with the requested shutdown order:

```go
bg1 := StartJob1()
bg2 := startJob2()
bg3 := startJob3()

// bg is the merged bg1, bg2 and bg3; bg3 shuts down after bg1 and bg2
bg := bg3.ShutdownAfter(bg1, bg2)
```

### Recommendations

Programs that use Background should follow these rules to keep interfaces consistent:

1. All functions that initialize application-scoped background jobs should return `Background` as its **last return value**.
2. If an error can occur during initialization it is still should be returned as `Background` using function `WithError`.
3. Never return nil `Background` - return `Empty()` instead, or do not return `Background` at all if it is not needed.
4. Every background job should be shutdownable and/or waitable.

There might be special cases, when returning background as the last return value is not possible, for example - when using dependency injection packages. To handle this case, embed `Background` into dependency's return value:

```go
package app

import "github.com/lefelys/background"

type Server struct {
	background.Background

	server *http.Server
}

type Updater interface {
	background.Background

	Update() error
}

func NewApp(server *Server, updater Updater) background.Background {
	 bg := server.ShutdownAfter(updater)

	 /*...*/
}
```

## Examples

See [examples](examples/)
