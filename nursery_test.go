package nursery_test

import (
	"context"
	"errors"
	"log"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/arunsworld/nursery"
)

func ExampleConcurrentJob() {
	nursery.RunConcurrently(
		// Job 1
		func(context.Context, chan error) {
			time.Sleep(time.Millisecond * 10)
			log.Println("Job 1 done...")
		},
		// Job 2
		func(context.Context, chan error) {
			time.Sleep(time.Millisecond * 5)
			log.Println("Job 2 done...")
		},
	)
	log.Println("All jobs done...")
}

func TestRunConcurrently(t *testing.T) {
	t.Run("jobs without errors - we wait for the longest running one", func(t *testing.T) {
		jobsDone := [3]bool{}

		jobFastest := func(context.Context, chan error) { jobsDone[0] = true }
		jobSlower := func(context.Context, chan error) { time.Sleep(time.Millisecond); jobsDone[1] = true }
		jobSlowest := func(context.Context, chan error) { time.Sleep(time.Millisecond * 5); jobsDone[2] = true }

		jobs := []nursery.ConcurrentJob{
			jobSlower, jobSlowest, jobFastest,
		}

		err := nursery.RunConcurrently(jobs...)
		if err != nil {
			t.Fatal(err)
		}

		if jobsDone != [3]bool{true, true, true} {
			t.Fatalf("expected all jobs to be done but instead got: %v", jobsDone)
		}

		// Now in a difference sequence
		jobs = []nursery.ConcurrentJob{
			jobFastest, jobSlower, jobSlowest,
		}

		err = nursery.RunConcurrently(jobs...)
		if err != nil {
			t.Fatal(err)
		}

		if jobsDone != [3]bool{true, true, true} {
			t.Fatalf("expected all jobs to be done but instead got: %v", jobsDone)
		}
	})

	t.Run("jobs with one err - error handled; we ensure all jobs are cleaned up", func(t *testing.T) {
		jobsDone := [4]bool{}
		jobsOutput := [4]int{}

		jobFastest := func(context.Context, chan error) {
			jobsOutput[2]++
			jobsDone[2] = true
		}
		jobSlower := func(context.Context, chan error) {
			time.Sleep(time.Millisecond)
			jobsOutput[0]++
			jobsDone[0] = true
		}
		jobSlowest := func(context.Context, chan error) {
			time.Sleep(time.Millisecond * 5)
			jobsOutput[1]++
			jobsDone[1] = true
		}
		slowerJobWithErr := func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond)
			ch <- errors.New("slowerJobWithErr error")
			jobsDone[3] = true
		}

		err := nursery.RunConcurrently(jobSlower, jobSlowest, jobFastest, slowerJobWithErr)

		if jobsDone != [4]bool{true, true, true, true} {
			t.Fatalf("expected all jobs to be done but instead got: %v", jobsDone)
		}

		if err == nil {
			t.Fatal("expected to have received an error but didn't")
		}

		if err.Error() != "slowerJobWithErr error" {
			t.Fatal("Error not as expected")
		}

	})

	t.Run("jobs with one err - ensure long jobs have an opportunity to bail early", func(t *testing.T) {
		jobsDone := [2]bool{}
		jobsOutput := [2]int{}

		jobWithErr := func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 2)
			ch <- errors.New("jobWithErr error")
			jobsDone[0] = true
		}
		neverEndingJob := func(ctx context.Context, ch chan error) {
			func() {
				for {
					select {
					case <-ctx.Done():
						return
					default:
						time.Sleep(time.Millisecond)
						jobsOutput[1]++
					}
				}
			}()
			jobsDone[1] = true
		}

		err := nursery.RunConcurrently(jobWithErr, neverEndingJob)

		if jobsDone != [2]bool{true, true} {
			t.Fatalf("expected all jobs to be done but instead got: %v", jobsDone)
		}

		if err == nil {
			t.Fatal("expected to have received an error but didn't")
		}

		if jobsOutput[1] < 1 || jobsOutput[1] > 3 {
			t.Fatal("expected to have bailed early but didn't")
		}
	})

	t.Run("jobs with multiple errors - everything continues to work as expected", func(t *testing.T) {
		jobsDone := [3]bool{}
		jobsOutput := [3]int{}

		jobWithErr := func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 2)
			ch <- errors.New("jobWithErr error")
			jobsDone[0] = true
		}
		jobWithAnotherErr := func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 4)
			ch <- errors.New("jobWithAnotherErr error")
			jobsDone[2] = true
		}
		neverEndingJob := func(ctx context.Context, ch chan error) {
			func() {
				for {
					select {
					case <-ctx.Done():
						return
					default:
						time.Sleep(time.Millisecond)
						jobsOutput[1]++
					}
				}
			}()
			jobsDone[1] = true
		}

		err := nursery.RunConcurrently(jobWithErr, neverEndingJob, jobWithAnotherErr)

		if jobsDone != [3]bool{true, true, true} {
			t.Fatalf("expected all jobs to be done but instead got: %v", jobsDone)
		}

		if err == nil {
			t.Fatal("expected to have received an error but didn't")
		}

		if jobsOutput[1] < 1 || jobsOutput[1] > 3 {
			t.Fatal("expected to have bailed early but didn't")
		}
	})
}

func TestRunUntilFirstCompletion(t *testing.T) {
	t.Run("jobs without errors - we wait for the short running one", func(t *testing.T) {
		jobsDone := [2]bool{}

		jobFastest := func(context.Context, chan error) { jobsDone[0] = true }
		jobForever := func(ctx context.Context, errCh chan error) {
			delay := time.NewTimer(time.Second * 500)
			select {
			case <-ctx.Done():
				delay.Stop()
			case <-delay.C:
			}
			jobsDone[1] = true
		}

		err := nursery.RunUntilFirstCompletion(jobFastest, jobForever)
		if err != nil {
			t.Fatal(err)
		}

		if jobsDone != [2]bool{true, true} {
			t.Fatalf("expected all jobs to be done but instead got: %v", jobsDone)
		}
	})

	// RunUntilFirstCompletion should also satisfy err cases identical to RunConcurrently
	t.Run("jobs with one err - ensure long jobs have an opportunity to bail early", func(t *testing.T) {
		jobsDone := [2]bool{}
		jobsOutput := [2]int{}

		jobWithErr := func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 2)
			ch <- errors.New("jobWithErr error")
			jobsDone[0] = true
		}
		neverEndingJob := func(ctx context.Context, ch chan error) {
			func() {
				for {
					select {
					case <-ctx.Done():
						return
					default:
						time.Sleep(time.Millisecond)
						jobsOutput[1]++
					}
				}
			}()
			jobsDone[1] = true
		}

		err := nursery.RunUntilFirstCompletion(jobWithErr, neverEndingJob)

		if jobsDone != [2]bool{true, true} {
			t.Fatalf("expected all jobs to be done but instead got: %v", jobsDone)
		}

		if err == nil {
			t.Fatal("expected to have received an error but didn't")
		}

		if jobsOutput[1] < 1 || jobsOutput[1] > 3 {
			t.Fatal("expected to have bailed early but didn't")
		}
	})

	t.Run("jobs with multiple errors - everything continues to work as expected", func(t *testing.T) {
		jobsDone := [3]bool{}
		jobsOutput := [3]int{}

		jobWithErr := func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 2)
			ch <- errors.New("jobWithErr error")
			jobsDone[0] = true
		}
		jobWithAnotherErr := func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 4)
			ch <- errors.New("jobWithAnotherErr error")
			jobsDone[2] = true
		}
		neverEndingJob := func(ctx context.Context, ch chan error) {
			func() {
				for {
					select {
					case <-ctx.Done():
						return
					default:
						time.Sleep(time.Millisecond)
						jobsOutput[1]++
					}
				}
			}()
			jobsDone[1] = true
		}

		err := nursery.RunUntilFirstCompletion(jobWithErr, neverEndingJob, jobWithAnotherErr)

		if jobsDone != [3]bool{true, true, true} {
			t.Fatalf("expected all jobs to be done but instead got: %v", jobsDone)
		}

		if err == nil {
			t.Fatal("expected to have received an error but didn't")
		}

		if jobsOutput[1] < 1 || jobsOutput[1] > 3 {
			t.Fatal("expected to have bailed early but didn't")
		}
	})
}

func TestRunConcurrentlyWithTimeout(t *testing.T) {
	t.Run("jobs without errors - timeout stops running processes", func(t *testing.T) {
		jobsDone := [2]bool{}
		jobsCount := [2]int{}

		jobForeverA := func(ctx context.Context, errCh chan error) {
			ticker := time.NewTicker(time.Millisecond)
			func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						jobsCount[0]++
					}
				}
			}()
			ticker.Stop()
			jobsDone[0] = true
		}
		jobForeverB := func(ctx context.Context, errCh chan error) {
			ticker := time.NewTicker(time.Millisecond)
			func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						jobsCount[1]++
					}
				}
			}()
			ticker.Stop()
			jobsDone[1] = true
		}

		err := nursery.RunConcurrentlyWithTimeout(time.Millisecond*10, jobForeverA, jobForeverB)
		if err != nil {
			log.Fatal(err)
		}

		if jobsDone != [2]bool{true, true} {
			t.Fatalf("expected all jobs to be done but instead got: %v", jobsDone)
		}

		if jobsCount[0]+jobsCount[1] < 10 || jobsCount[0]+jobsCount[1] > 22 {
			t.Fatalf("jobsCount out of range. Expected 10 < total < 22 but got: %v", jobsCount)
		}
	})

	t.Run("jobs without errors - jobs finish before timeout", func(t *testing.T) {
		jobsDone := [2]bool{}
		jobsCount := [2]int{}

		quickJobA := func(ctx context.Context, errCh chan error) {
			ticker := time.NewTicker(time.Millisecond)
			func() {
				for i := 0; i < 5; i++ {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						jobsCount[0]++
					}
				}
			}()
			ticker.Stop()
			jobsDone[0] = true
		}
		notAsQuickJobB := func(ctx context.Context, errCh chan error) {
			ticker := time.NewTicker(time.Millisecond)
			func() {
				for i := 0; i < 10; i++ {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						jobsCount[1]++
					}
				}
			}()
			ticker.Stop()
			jobsDone[1] = true
		}

		err := nursery.RunConcurrentlyWithTimeout(time.Second, quickJobA, notAsQuickJobB)
		if err != nil {
			log.Fatal(err)
		}

		if jobsDone != [2]bool{true, true} {
			t.Fatalf("expected all jobs to be done but instead got: %v", jobsDone)
		}

		if jobsCount[0]+jobsCount[1] < 13 || jobsCount[0]+jobsCount[1] > 17 {
			t.Fatalf("jobsCount out of range. Expected 13 < total < 17 but got: %v", jobsCount)
		}
	})

	// RunUntilFirstCompletion should also satisfy err cases identical to RunConcurrently
	t.Run("jobs with one err - ensure long jobs have an opportunity to bail early", func(t *testing.T) {
		jobsDone := [2]bool{}
		jobsOutput := [2]int{}

		jobWithErr := func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 2)
			ch <- errors.New("jobWithErr error")
			jobsDone[0] = true
		}
		neverEndingJob := func(ctx context.Context, ch chan error) {
			func() {
				for {
					select {
					case <-ctx.Done():
						return
					default:
						time.Sleep(time.Millisecond)
						jobsOutput[1]++
					}
				}
			}()
			jobsDone[1] = true
		}

		err := nursery.RunConcurrentlyWithTimeout(time.Second, jobWithErr, neverEndingJob)

		if jobsDone != [2]bool{true, true} {
			t.Fatalf("expected all jobs to be done but instead got: %v", jobsDone)
		}

		if err == nil {
			t.Fatal("expected to have received an error but didn't")
		}

		if jobsOutput[1] < 1 || jobsOutput[1] > 3 {
			t.Fatal("expected to have bailed early but didn't")
		}
	})

	t.Run("jobs with multiple errors - everything continues to work as expected", func(t *testing.T) {
		jobsDone := [3]bool{}
		jobsOutput := [3]int{}

		jobWithErr := func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 2)
			ch <- errors.New("jobWithErr error")
			jobsDone[0] = true
		}
		jobWithAnotherErr := func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 4)
			ch <- errors.New("jobWithAnotherErr error")
			jobsDone[2] = true
		}
		neverEndingJob := func(ctx context.Context, ch chan error) {
			func() {
				for {
					select {
					case <-ctx.Done():
						return
					default:
						time.Sleep(time.Millisecond)
						jobsOutput[1]++
					}
				}
			}()
			jobsDone[1] = true
		}

		err := nursery.RunConcurrentlyWithTimeout(time.Second, jobWithErr, neverEndingJob, jobWithAnotherErr)

		if jobsDone != [3]bool{true, true, true} {
			t.Fatalf("expected all jobs to be done but instead got: %v", jobsDone)
		}

		if err == nil {
			t.Fatal("expected to have received an error but didn't")
		}

		if jobsOutput[1] < 1 || jobsOutput[1] > 3 {
			t.Fatal("expected to have bailed early but didn't")
		}
	})
}

func TestRunUntilFirstCompletionWithTimeout(t *testing.T) {
	t.Run("jobs without errors - timeout stops running processes", func(t *testing.T) {
		jobsDone := [2]bool{}
		jobsCount := [2]int{}

		jobForeverA := func(ctx context.Context, errCh chan error) {
			ticker := time.NewTicker(time.Millisecond)
			func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						jobsCount[0]++
					}
				}
			}()
			ticker.Stop()
			jobsDone[0] = true
		}
		jobForeverB := func(ctx context.Context, errCh chan error) {
			ticker := time.NewTicker(time.Millisecond)
			func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						jobsCount[1]++
					}
				}
			}()
			ticker.Stop()
			jobsDone[1] = true
		}

		err := nursery.RunUntilFirstCompletionWithTimeout(time.Millisecond*10, jobForeverA, jobForeverB)
		if err != nil {
			log.Fatal(err)
		}

		if jobsDone != [2]bool{true, true} {
			t.Fatalf("expected all jobs to be done but instead got: %v", jobsDone)
		}

		if jobsCount[0]+jobsCount[1] < 18 || jobsCount[0]+jobsCount[1] > 22 {
			t.Fatalf("jobsCount out of range. Expected 18 < total < 22 but got: %v", jobsCount)
		}
	})

	t.Run("jobs without errors - quick job finishes before timeout", func(t *testing.T) {
		jobsDone := [2]bool{}
		jobsCount := [2]int{}

		quickJobA := func(ctx context.Context, errCh chan error) {
			ticker := time.NewTicker(time.Millisecond)
			func() {
				for i := 0; i < 5; i++ {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						jobsCount[0]++
					}
				}
			}()
			ticker.Stop()
			jobsDone[0] = true
		}
		notAsQuickJobB := func(ctx context.Context, errCh chan error) {
			ticker := time.NewTicker(time.Millisecond)
			func() {
				for i := 0; i < 100; i++ {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						jobsCount[1]++
					}
				}
			}()
			ticker.Stop()
			jobsDone[1] = true
		}

		err := nursery.RunUntilFirstCompletionWithTimeout(time.Millisecond*10, quickJobA, notAsQuickJobB)
		if err != nil {
			log.Fatal(err)
		}

		if jobsDone != [2]bool{true, true} {
			t.Fatalf("expected all jobs to be done but instead got: %v", jobsDone)
		}

		if jobsCount[0]+jobsCount[1] < 8 || jobsCount[0]+jobsCount[1] > 12 {
			t.Fatalf("jobsCount out of range. Expected 8 < total < 12 but got: %v", jobsCount)
		}
	})

	// RunUntilFirstCompletion should also satisfy err cases identical to RunConcurrently
	t.Run("jobs with one err - ensure long jobs have an opportunity to bail early", func(t *testing.T) {
		jobsDone := [2]bool{}
		jobsOutput := [2]int{}

		jobWithErr := func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 2)
			ch <- errors.New("jobWithErr error")
			jobsDone[0] = true
		}
		neverEndingJob := func(ctx context.Context, ch chan error) {
			func() {
				for {
					select {
					case <-ctx.Done():
						return
					default:
						time.Sleep(time.Millisecond)
						jobsOutput[1]++
					}
				}
			}()
			jobsDone[1] = true
		}

		err := nursery.RunUntilFirstCompletionWithTimeout(time.Second, jobWithErr, neverEndingJob)

		if jobsDone != [2]bool{true, true} {
			t.Fatalf("expected all jobs to be done but instead got: %v", jobsDone)
		}

		if err == nil {
			t.Fatal("expected to have received an error but didn't")
		}

		if jobsOutput[1] < 1 || jobsOutput[1] > 3 {
			t.Fatal("expected to have bailed early but didn't")
		}
	})

	t.Run("jobs with multiple errors - everything continues to work as expected", func(t *testing.T) {
		jobsDone := [3]bool{}
		jobsOutput := [3]int{}

		jobWithErr := func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 2)
			ch <- errors.New("jobWithErr error")
			jobsDone[0] = true
		}
		jobWithAnotherErr := func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 4)
			ch <- errors.New("jobWithAnotherErr error")
			jobsDone[2] = true
		}
		neverEndingJob := func(ctx context.Context, ch chan error) {
			func() {
				for {
					select {
					case <-ctx.Done():
						return
					default:
						time.Sleep(time.Millisecond)
						jobsOutput[1]++
					}
				}
			}()
			jobsDone[1] = true
		}

		jobs := []nursery.ConcurrentJob{
			jobWithErr, neverEndingJob, jobWithAnotherErr,
		}

		err := nursery.RunUntilFirstCompletionWithTimeout(time.Second, jobs...)

		if jobsDone != [3]bool{true, true, true} {
			t.Fatalf("expected all jobs to be done but instead got: %v", jobsDone)
		}

		if err == nil {
			t.Fatal("expected to have received an error but didn't")
		}

		if jobsOutput[1] < 1 || jobsOutput[1] > 3 {
			t.Fatal("expected to have bailed early but didn't")
		}
	})
}

func TestRunConcurrentlyWithContext(t *testing.T) {
	t.Run("jobs without errors - parent context cancellation stops running processes", func(t *testing.T) {
		jobsDone := [2]bool{}
		jobsCount := [2]int{}

		jobForeverA := func(ctx context.Context, errCh chan error) {
			ticker := time.NewTicker(time.Millisecond)
			func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						jobsCount[0]++
					}
				}
			}()
			ticker.Stop()
			jobsDone[0] = true
		}
		jobForeverB := func(ctx context.Context, errCh chan error) {
			ticker := time.NewTicker(time.Millisecond)
			func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						jobsCount[1]++
					}
				}
			}()
			ticker.Stop()
			jobsDone[1] = true
		}

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(time.Millisecond * 10)
			cancel()
		}()

		err := nursery.RunConcurrentlyWithContext(ctx, jobForeverA, jobForeverB)
		if err != nil {
			log.Fatal(err)
		}

		if jobsDone != [2]bool{true, true} {
			t.Fatalf("expected all jobs to be done but instead got: %v", jobsDone)
		}

		if jobsCount[0]+jobsCount[1] < 10 || jobsCount[0]+jobsCount[1] > 25 {
			t.Fatalf("jobsCount out of range. Expected 10 < total < 25 but got: %v", jobsCount)
		}
	})
}

func TestRunUntilFirstCompletionWithContext(t *testing.T) {
	t.Run("jobs without errors - parent context cancellation stops running processes", func(t *testing.T) {
		jobsDone := [2]bool{}
		jobsCount := [2]int{}

		jobForeverA := func(ctx context.Context, errCh chan error) {
			ticker := time.NewTicker(time.Millisecond)
			func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						jobsCount[0]++
					}
				}
			}()
			ticker.Stop()
			jobsDone[0] = true
		}
		jobForeverB := func(ctx context.Context, errCh chan error) {
			ticker := time.NewTicker(time.Millisecond)
			func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						jobsCount[1]++
					}
				}
			}()
			ticker.Stop()
			jobsDone[1] = true
		}

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(time.Millisecond * 10)
			cancel()
		}()

		err := nursery.RunUntilFirstCompletionWithContext(ctx, jobForeverA, jobForeverB)
		if err != nil {
			log.Fatal(err)
		}

		if jobsDone != [2]bool{true, true} {
			t.Fatalf("expected all jobs to be done but instead got: %v", jobsDone)
		}

		if jobsCount[0]+jobsCount[1] < 18 || jobsCount[0]+jobsCount[1] > 25 {
			t.Fatalf("jobsCount out of range. Expected 18 < total < 25 but got: %v", jobsCount)
		}
	})
}

func TestRunMultipleCopiesConcurrently(t *testing.T) {
	t.Run("producer and multiple concurrent consumers", func(t *testing.T) {
		ch := make(chan struct{})
		result := make(map[int]int)
		nursery.RunConcurrently(
			// producer job producing 10 items
			func(context.Context, chan error) {
				for i := 0; i < 10; i++ {
					ch <- struct{}{}
				}
				close(ch)
			},
			// consumer job
			func(context.Context, chan error) {
				mu := sync.Mutex{}
				nursery.RunMultipleCopiesConcurrently(5,
					// the job listens on ch and if activated will update
					// result map with a counter for it's own Job ID
					func(ctx context.Context, errCh chan error) {
						myJobID := ctx.Value(nursery.JobID).(int) + 1
						for range ch {
							time.Sleep(time.Millisecond * 10)
							mu.Lock()
							v := result[myJobID]
							result[myJobID] = v + 1
							mu.Unlock()
						}
					},
				)
			},
		)
		if !reflect.DeepEqual(result, map[int]int{
			1: 2, 2: 2, 3: 2, 4: 2, 5: 2,
		}) {
			t.Fatalf("expected a different result than: %#v", result)
		}
	})
}

func TestIsContextDone(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	t.Run("detects contexts that are alive (not done)", func(t *testing.T) {
		// Then
		status := nursery.IsContextDone(ctx)
		if status == true {
			t.Fatal("expected context not to be done, but it is")
		}
	})
	t.Run("detects contexts that are not alive (done or cancelled)", func(t *testing.T) {
		// When
		cancel()
		// Then
		status := nursery.IsContextDone(ctx)
		if status == false {
			t.Fatal("expected context to be done, but it is not")
		}
	})
}
