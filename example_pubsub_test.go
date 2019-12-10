package nursery

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

func Example() {
	ch := make(chan int)
	err := RunConcurrently(
		// producer job: produce numbers into ch and once done close it
		func(ctx context.Context, errCh chan error) {
			produceNumbers(ctx, ch)
			close(ch)
		},
		// consumer job
		func(ctx context.Context, errCh chan error) {
			// run 5 copies of the consumer reading from ch until closed or err encountered
			err := RunMultipleCopiesConcurrentlyWithContext(ctx, 5,
				func(ctx context.Context, errCh chan error) {
					if err := consumeNumbers(ctx, ch); err != nil {
						errCh <- err
					}
				},
			)
			if err != nil {
				errCh <- err
				// drain the channel to not block the producer in the event of an error
				for range ch {
				}
			}
		},
	)
	if err != nil {
		log.Fatal(err)
	}
}

func produceNumbers(ctx context.Context, ch chan int) {
	for i := 0; i < 200; i++ {
		select {
		case <-ctx.Done():
			fmt.Printf("producer terminating early after sending numbers up to: %d\n", i)
			return
		default:
			time.Sleep(time.Nanosecond * 100)
			ch <- i
		}
	}
	fmt.Println("all numbers produced... now exiting...")
}

func consumeNumbers(ctx context.Context, ch chan int) error {
	jobID := ctx.Value(JobID).(int)
	for v := range ch {
		select {
		case <-ctx.Done():
			fmt.Printf("Job %d terminating early\n", jobID)
			return nil
		default:
			if v == 10 {
				fmt.Printf("Job %d received value 10 which is an error\n", jobID)
				return errors.New("number 10 received")
			}
			fmt.Printf("Job %d received value: %d\n", jobID, v)
			time.Sleep(time.Millisecond * 10)
		}
	}
	fmt.Printf("Job %d finishing up...\n", jobID)
	return nil
}
