package background

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

func ExampleWithShutdown() {
	bg := func() Background {
		bg, tail := WithShutdown()
		ticker := time.NewTicker(1 * time.Second)

		go func() {
			for {
				select {
				// receive a shutdown signal
				case <-tail.End():
					ticker.Stop()

					// send a signal that the shutdown is complete
					tail.Done()

					return
				case <-ticker.C:
					// some job
				}
			}
		}()

		return bg
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := bg.Shutdown(ctx)
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleWithShutdown_dependency() {
	runJob := func(name string) Background {
		bg, tail := WithShutdown()

		go func() {
			<-tail.End()

			fmt.Println("shutdown " + name)

			tail.Done()
		}()

		return bg
	}

	bg1 := runJob("job 1")
	bg2 := runJob("job 2")
	bg3 := runJob("job 3")

	// bg3 will be shut down first, then bg2, then bg1
	bg := bg1.DependsOn(bg2).DependsOn(bg3)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := bg.Shutdown(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Output: shutdown job 3
	// shutdown job 2
	// shutdown job 1
}

func ExampleWithShutdown_dependencyWrap() {
	bg1 := func() Background {
		bg, tail := WithShutdown()

		go func() {
			<-tail.End()
			fmt.Println("shutdown job 1")
			tail.Done()
		}()

		return bg
	}()

	// bg1 will be shut down first, then bg2
	bg2, tail := WithShutdown(bg1)

	go func() {
		<-tail.End()
		fmt.Println("shutdown job 2")
		tail.Done()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := bg2.Shutdown(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Output: shutdown job 1
	// shutdown job 2
}

func ExampleWithWait() {
	bg := func() Background {
		bg, tail := WithWait()

		for i := 1; i <= 3; i++ {
			tail.Add(1)

			go func(i int) {
				<-time.After(time.Duration(i) * 50 * time.Millisecond)
				fmt.Printf("job %d ended\n", i)

				tail.Done()
			}(i)
		}

		return bg
	}()

	// blocks until Background's WaitGroup counter is zero
	bg.Wait()

	// Output: job 1 ended
	// job 2 ended
	// job 3 ended
}

func ExampleWithError() {
	bg := func() Background {
		return WithError(errors.New("error"))
	}()

	if err := bg.Err(); err != nil {
		fmt.Println(err)
	}

	// Output: error
}

func ExampleWithErrorGroup() {
	bg := func() Background {
		bg, tail := WithErrorGroup()

		go func() {
			tail.Error(errors.New("error"))
		}()

		return bg
	}()

	time.Sleep(100 * time.Millisecond)

	if err := bg.Err(); err != nil {
		fmt.Println(err)
	}

	// Output: error
}

func ExampleWithAnnotation() {
	bg := func() Background {
		bg, tail := WithErrorGroup()

		go func() {
			tail.Error(errors.New("error"))
		}()

		return WithAnnotation("my job", bg)
	}()

	time.Sleep(100 * time.Millisecond)

	if err := bg.Err(); err != nil {
		fmt.Println(err)
	}

	// Output: my job: error
}

func ExampleWithAnnotation_shutdown() {
	bg := func() Background {
		bg, tail := WithShutdown()

		go func() {
			<-tail.End()
			<-time.After(1 * time.Second)
			tail.Done()
		}()

		return WithAnnotation("my job", bg)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	err := bg.Shutdown(ctx)
	if err != nil {
		fmt.Println(err)
	}

	// Output: my job: timeout expired
}

func ExampleMerge() {
	runJob := func(name string, duration time.Duration) Background {
		bg, tail := WithShutdown()

		go func() {
			<-tail.End()

			<-time.After(duration)
			fmt.Println("shutdown " + name)

			tail.Done()
		}()

		return bg
	}

	bg1 := runJob("job 1", 50*time.Millisecond)
	bg2 := runJob("job 2", 100*time.Millisecond)
	bg3 := runJob("job 3", 150*time.Millisecond)

	bg := Merge(bg1, bg2, bg3)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := bg.Shutdown(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Output: shutdown job 1
	// shutdown job 2
	// shutdown job 3
}

func ExampleWithValue() {
	type key int

	var greetingKey key

	getGreeting := func(bg Background) (c chan string, ok bool) {
		value := bg.Value(greetingKey)
		if value != nil {
			return value.(chan string), true
		}

		return
	}

	bg := func() Background {
		c := make(chan string)
		bg := WithValue(greetingKey, c)

		go func() {
			c <- "hi"
		}()

		return bg
	}()

	c, ok := getGreeting(bg)
	if ok {
		fmt.Println(<-c)
	}

	// Output: hi
}
