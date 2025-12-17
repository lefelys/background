package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lefelys/background"
)

type Generator struct{}

func NewGenerator() (out chan time.Time, bg background.Background) {
	c := Generator{}
	out, bg = c.Start()

	return out, background.WithAnnotation("generator", bg)
}

func (c *Generator) Start() (chan time.Time, background.Background) {
	bg, tail := background.WithShutdown()
	ticker := time.NewTicker(1 * time.Second)
	out := make(chan time.Time)

	go func() {
		for {
			select {
			case <-tail.End():
				ticker.Stop()
				fmt.Println("generator shutdown")
				tail.Done()
				return
			case out <- <-ticker.C:
			}
		}
	}()

	return out, bg
}

type Processor struct{}

func NewProcessor(in <-chan time.Time) background.Background {
	p := Processor{}
	bg := p.Start(in)

	return background.WithAnnotation("processor", bg)
}

func (p *Processor) Start(in <-chan time.Time) (bg background.Background) {
	bg, tail := background.WithShutdown()

	go func() {
		for {
			select {
			case <-tail.End():
				fmt.Println("processor shutdown")
				tail.Done()
				return
			case t := <-in:
				fmt.Println(t)
			}
		}
	}()

	return
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

type key int

var fatalKey key

func getServerFatalCh(bg background.Background) chan error {
	value := bg.Value(fatalKey)
	if value != nil {
		return value.(chan error)
	}

	return nil
}

func (s *Server) Start() background.Background {
	shutdownBg, shutdownTail := background.WithShutdown()
	errBg, errTail := background.WithErrorGroup()
	fatal := make(chan error)
	serverFatalBg := background.WithValue(fatalKey, fatal)

	go func() {
		err := s.Server.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			fatal <- err
		}
	}()

	go func() {
		<-shutdownTail.End()

		// context.Background() never expires, so Server's Shutdown call may
		// only return errors from closing Server's underlying Listener(s).
		err := s.Server.Shutdown(context.Background())
		if err != nil {
			errTail.Errorf("shutdown error from server: %w", err)
		}
		fmt.Println("server shutdown")
		shutdownTail.Done()
	}()

	return background.Merge(shutdownBg, errBg, serverFatalBg)
}

func main() {
	out, generatorBg := NewGenerator()
	if err := generatorBg.Err(); err != nil {
		log.Fatal(err)
	}

	processorBg := NewProcessor(out)
	if err := processorBg.Err(); err != nil {
		log.Fatal(err)
	}

	serverBg := NewServer()
	if err := serverBg.Err(); err != nil {
		log.Fatal(err)
	}

	// generator will be shut down first, then processor, then server
	appBackground := serverBg.
		DependsOn(processorBg).
		DependsOn(generatorBg)

	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case err := <-getServerFatalCh(appBackground):
		log.Fatalf("fatal error: %v", err)
	case <-shutdownSig:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := appBackground.Shutdown(ctx)
		if err != nil {
			log.Fatal(err)
		}

		if err := appBackground.Err(); err != nil {
			log.Fatal(err)
		}
	}
}
