package nursery

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

func Example() {
	ch := make(chan int)
	err := RunConcurrently([]ConcurrentJob{
		producerJob(ch),
		consumerJob(ch, 5),
	})
	if err != nil {
		log.Fatal(err)
	}
}

func producerJob(ch chan int) ConcurrentJobFunc {
	return func(ctx context.Context, errCh chan error) {
		defer close(ch)

		for i := 0; i < 20; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(time.Nanosecond * 100)
				ch <- i
			}
		}
	}
}

func consumerJob(ch chan int, count int) ConcurrentJobFunc {
	return func(ctx context.Context, errCh chan error) {
		// we kick off count consumers - we could have wrapped them as ConcurrentJob
		// and used RunConcurrently - but that would be overuse and make things less readable
		wg := sync.WaitGroup{}
		for i := 0; i < count; i++ {
			wg.Add(1)
			go func(i int) {
				consume(ctx, i+1, ch, errCh)
				wg.Done()
			}(i)
		}
		wg.Wait()
		// In case consumers bailed out early due to errors ensure we're draining the
		// producer challenge to unblock the producer
		for range ch {
		}
	}
}

func consume(ctx context.Context, name int, ch chan int, errCh chan error) {
	for {
		select {
		case <-ctx.Done():
			return
		case v, ok := <-ch:
			if !ok {
				return
			}
			if v == 10 {
				errCh <- errors.New("number 10 received")
				return
			}
			time.Sleep(time.Millisecond)
			fmt.Printf("Job %d received value: %d\n", name, v)
		}
	}
}
