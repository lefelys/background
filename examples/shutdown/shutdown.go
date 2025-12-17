package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lefelys/background"
)

func StartJob(name string) background.Background {
	bg, tail := background.WithShutdown()
	ticker := time.NewTicker(1 * time.Second)

	go func() {
		for {
			select {
			case <-tail.End():
				ticker.Stop()
				fmt.Println("shutdown " + name)
				tail.Done()
				return
			case <-ticker.C:
				/*...*/
			}
		}
	}()

	return bg
}

func main() {
	bg1 := StartJob("job1")
	if err := bg1.Err(); err != nil {
		log.Fatal(err)
	}

	bg2 := StartJob("job2")
	if err := bg2.Err(); err != nil {
		log.Fatal(err)
	}

	// job2 will be shut down first, then job1
	appBg := bg1.DependsOn(bg2)

	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	<-shutdownSig

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := appBg.Shutdown(ctx)
	if err != nil {
		log.Fatal(err)
	}

	if err := appBg.Err(); err != nil {
		log.Fatal(err)
	}
}
