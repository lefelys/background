package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/lefelys/background"
)

type Fatality interface {
	Fatal() <-chan error
}

type Fatal struct {
	errCh chan error
}

func (f Fatal) Error(err error) {
	f.errCh <- err
}

func (f Fatal) Errorf(format string, a ...interface{}) {
	f.errCh <- fmt.Errorf(format, a...)
}

func (f Fatal) Fatal() <-chan error {
	return f.errCh
}

type key int

var fatalKey key

// WithFatality returns Background with Fatality value set and its tail.
// If Fatal Background already present in children - returns merged children
// and its tail
func WithFatality(children ...background.Background) (background.Background, background.ErrTail) {
	for _, child := range children {
		if f, ok := FatalityFromBackground(child); ok {
			return background.Merge(children...), f.(background.ErrTail)
		}
	}

	fatal := &Fatal{
		errCh: make(chan error),
	}

	return background.WithValue(fatalKey, fatal, children...), fatal
}

// FatalityFromBackground returns Fatality value stored in st. If there are
// multiple values associated with fatalKey in the tree, only
// the topmost and the leftmost will be returned.
func FatalityFromBackground(bg background.Background) (Fatality, bool) {
	f, ok := bg.Value(fatalKey).(Fatality)
	return f, ok
}

type Server struct {
	*http.Server
}

func NewServer() background.Background {
	server := &Server{
		Server: &http.Server{
			Addr:    ":8000",
			Handler: http.DefaultServeMux,
		},
	}

	bg := server.Start()

	return background.WithAnnotation("http server", bg)
}

func (s *Server) Start() background.Background {
	bg, fatalTail := WithFatality()

	go func() {
		err := s.Server.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			fatalTail.Errorf("server fatal error: %w", err)
		}
	}()

	return bg
}

func main() {
	serverBg := NewServer()
	if err := serverBg.Err(); err != nil {
		log.Fatal(err)
	}

	fatalBg, ok := FatalityFromBackground(serverBg)
	if !ok {
		log.Fatal("fatality background not found")
	}

	if err := <-fatalBg.Fatal(); err != nil {
		log.Fatal(err)
	}
}
