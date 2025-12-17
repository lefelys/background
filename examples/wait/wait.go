package main

import (
	"fmt"
	"time"

	"github.com/lefelys/background"
)

func main() {
	bg := background.Merge(StartJob1(), StartJob2())

	bg.Wait()

}

func StartJob1() background.Background {
	bg, tail := background.WithWait()

	for i := 0; i < 5; i++ {
		tail.Add(1)
		go func() {
			time.Sleep(1 * time.Second)
			fmt.Println("done job 1")
			tail.Done()
		}()
	}

	return bg
}

func StartJob2() background.Background {
	bg, tail := background.WithWait()

	for i := 0; i < 5; i++ {
		tail.Add(1)
		go func() {
			time.Sleep(2 * time.Second)
			fmt.Println("done job 2")
			tail.Done()
		}()
	}

	return bg
}
