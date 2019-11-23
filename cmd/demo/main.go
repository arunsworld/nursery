package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/arunsworld/nursery"
)

func main() {
	err := nursery.RunConcurrently([]nursery.ConcurrentJob{
		sleeper(3),
		sleeper(1),
		neverEndingUntilCancelled(),
	})
	if err != nil {
		log.Println(err)
	}

	err = nursery.RunUntilFirstCompletion([]nursery.ConcurrentJob{
		sleeper(3),
		sleeper(1),
		neverEndingUntilCancelled(),
	})
	if err != nil {
		log.Fatal(err)
	}
}

func sleeper(i int) nursery.ConcurrentJobFunc {
	return func(ctx context.Context, errCh chan error) {
		log.Printf("sleep %d started...", i)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second * time.Duration(int64(i))):
			log.Printf("sleep %d finished...", i)
			return
		case <-time.After(time.Second * 2):
			errCh <- fmt.Errorf("an error occurred in %d sleeper", i)
		}
	}
}

func neverEndingUntilCancelled() nursery.ConcurrentJobFunc {
	return func(ctx context.Context, errCh chan error) {
		<-ctx.Done()
	}
}
