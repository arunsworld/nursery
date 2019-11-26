package nursery

import (
	"context"
	"errors"
	"log"
	"testing"
	"time"
)

func ExampleConcurrentJobFunc() {
	var job ConcurrentJobFunc = func(ctx context.Context, errCh chan error) {
		delay := time.NewTimer(time.Second * 500)
		select {
		case <-ctx.Done():
			delay.Stop()
		case <-delay.C:
		}
	}

	RunConcurrently([]ConcurrentJob{job})
}

func TestRunConcurrently(t *testing.T) {
	t.Run("jobs without errors - we wait for the longest running one", func(t *testing.T) {
		jobsDone := [3]bool{}

		var jobFastest ConcurrentJobFunc = func(context.Context, chan error) { jobsDone[0] = true }
		var jobSlower ConcurrentJobFunc = func(context.Context, chan error) { time.Sleep(time.Millisecond); jobsDone[1] = true }
		var jobSlowest ConcurrentJobFunc = func(context.Context, chan error) { time.Sleep(time.Millisecond * 5); jobsDone[2] = true }

		jobs := []ConcurrentJob{
			jobSlower, jobSlowest, jobFastest,
		}

		err := RunConcurrently(jobs)
		if err != nil {
			t.Fatal(err)
		}

		if jobsDone != [3]bool{true, true, true} {
			t.Fatalf("expected all jobs to be done but instead got: %v", jobsDone)
		}

		// Now in a difference sequence
		jobs = []ConcurrentJob{
			jobFastest, jobSlower, jobSlowest,
		}

		err = RunConcurrently(jobs)
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

		var jobFastest ConcurrentJobFunc = func(context.Context, chan error) {
			jobsOutput[0]++
			jobsDone[0] = true
		}
		var jobSlower ConcurrentJobFunc = func(context.Context, chan error) {
			time.Sleep(time.Millisecond)
			jobsOutput[0]++
			jobsDone[1] = true
		}
		var jobSlowest ConcurrentJobFunc = func(context.Context, chan error) {
			time.Sleep(time.Millisecond * 5)
			jobsOutput[0]++
			jobsDone[2] = true
		}
		var slowerJobWithErr ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond)
			ch <- errors.New("slowerJobWithErr error")
			jobsDone[3] = true
		}

		jobs := []ConcurrentJob{
			jobSlower, jobSlowest, jobFastest, slowerJobWithErr,
		}

		err := RunConcurrently(jobs)

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

		var jobWithErr ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 2)
			ch <- errors.New("jobWithErr error")
			jobsDone[0] = true
		}
		var neverEndingJob ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
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

		jobs := []ConcurrentJob{
			jobWithErr, neverEndingJob,
		}

		err := RunConcurrently(jobs)

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

		var jobWithErr ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 2)
			ch <- errors.New("jobWithErr error")
			jobsDone[0] = true
		}
		var jobWithAnotherErr ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 4)
			ch <- errors.New("jobWithAnotherErr error")
			jobsDone[2] = true
		}
		var neverEndingJob ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
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

		jobs := []ConcurrentJob{
			jobWithErr, neverEndingJob, jobWithAnotherErr,
		}

		err := RunConcurrently(jobs)

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

		var jobFastest ConcurrentJobFunc = func(context.Context, chan error) { jobsDone[0] = true }
		var jobForever ConcurrentJobFunc = func(ctx context.Context, errCh chan error) {
			delay := time.NewTimer(time.Second * 500)
			select {
			case <-ctx.Done():
				delay.Stop()
			case <-delay.C:
			}
			jobsDone[1] = true
		}

		jobs := []ConcurrentJob{
			jobFastest, jobForever,
		}

		err := RunUntilFirstCompletion(jobs)
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

		var jobWithErr ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 2)
			ch <- errors.New("jobWithErr error")
			jobsDone[0] = true
		}
		var neverEndingJob ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
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

		jobs := []ConcurrentJob{
			jobWithErr, neverEndingJob,
		}

		err := RunUntilFirstCompletion(jobs)

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

		var jobWithErr ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 2)
			ch <- errors.New("jobWithErr error")
			jobsDone[0] = true
		}
		var jobWithAnotherErr ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 4)
			ch <- errors.New("jobWithAnotherErr error")
			jobsDone[2] = true
		}
		var neverEndingJob ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
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

		jobs := []ConcurrentJob{
			jobWithErr, neverEndingJob, jobWithAnotherErr,
		}

		err := RunUntilFirstCompletion(jobs)

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

		var jobForeverA ConcurrentJobFunc = func(ctx context.Context, errCh chan error) {
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
		var jobForeverB ConcurrentJobFunc = func(ctx context.Context, errCh chan error) {
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

		jobs := []ConcurrentJob{jobForeverA, jobForeverB}

		err := RunConcurrentlyWithTimeout(jobs, time.Millisecond*10)
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

	t.Run("jobs without errors - jobs finish before timeout", func(t *testing.T) {
		jobsDone := [2]bool{}
		jobsCount := [2]int{}

		var quickJobA ConcurrentJobFunc = func(ctx context.Context, errCh chan error) {
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
		var notAsQuickJobB ConcurrentJobFunc = func(ctx context.Context, errCh chan error) {
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

		jobs := []ConcurrentJob{quickJobA, notAsQuickJobB}

		err := RunConcurrentlyWithTimeout(jobs, time.Second)
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

		var jobWithErr ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 2)
			ch <- errors.New("jobWithErr error")
			jobsDone[0] = true
		}
		var neverEndingJob ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
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

		jobs := []ConcurrentJob{
			jobWithErr, neverEndingJob,
		}

		err := RunConcurrentlyWithTimeout(jobs, time.Second)

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

		var jobWithErr ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 2)
			ch <- errors.New("jobWithErr error")
			jobsDone[0] = true
		}
		var jobWithAnotherErr ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 4)
			ch <- errors.New("jobWithAnotherErr error")
			jobsDone[2] = true
		}
		var neverEndingJob ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
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

		jobs := []ConcurrentJob{
			jobWithErr, neverEndingJob, jobWithAnotherErr,
		}

		err := RunConcurrentlyWithTimeout(jobs, time.Second)

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

		var jobForeverA ConcurrentJobFunc = func(ctx context.Context, errCh chan error) {
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
		var jobForeverB ConcurrentJobFunc = func(ctx context.Context, errCh chan error) {
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

		jobs := []ConcurrentJob{jobForeverA, jobForeverB}

		err := RunUntilFirstCompletionWithTimeout(jobs, time.Millisecond*10)
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

		var quickJobA ConcurrentJobFunc = func(ctx context.Context, errCh chan error) {
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
		var notAsQuickJobB ConcurrentJobFunc = func(ctx context.Context, errCh chan error) {
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

		jobs := []ConcurrentJob{quickJobA, notAsQuickJobB}

		err := RunUntilFirstCompletionWithTimeout(jobs, time.Millisecond*10)
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

		var jobWithErr ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 2)
			ch <- errors.New("jobWithErr error")
			jobsDone[0] = true
		}
		var neverEndingJob ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
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

		jobs := []ConcurrentJob{
			jobWithErr, neverEndingJob,
		}

		err := RunUntilFirstCompletionWithTimeout(jobs, time.Second)

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

		var jobWithErr ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 2)
			ch <- errors.New("jobWithErr error")
			jobsDone[0] = true
		}
		var jobWithAnotherErr ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
			time.Sleep(time.Millisecond * 4)
			ch <- errors.New("jobWithAnotherErr error")
			jobsDone[2] = true
		}
		var neverEndingJob ConcurrentJobFunc = func(ctx context.Context, ch chan error) {
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

		jobs := []ConcurrentJob{
			jobWithErr, neverEndingJob, jobWithAnotherErr,
		}

		err := RunUntilFirstCompletionWithTimeout(jobs, time.Second)

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
