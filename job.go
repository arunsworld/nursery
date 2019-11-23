package nursery

import (
	"context"
	"sync"
)

// ConcurrentJob defines the behavior of a concurrent Job
type ConcurrentJob interface {
	Start(context.Context, chan error)
}

// ConcurrentJobFunc is a function of type ConcurrentJob
type ConcurrentJobFunc func(context.Context, chan error)

// Start implemented to satisfy the ConcurrentJob interface
func (cjf ConcurrentJobFunc) Start(ctx context.Context, errCh chan error) {
	cjf(ctx, errCh)
}

// RunConcurrently runs jobs concurrently
func RunConcurrently(jobs []ConcurrentJob) error {
	var result error

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 10)
	waitForErrCompletion := sync.WaitGroup{}
	waitForErrCompletion.Add(1)
	go func() {
		result = monitorErrChForFirstError(cancel, errCh)
		waitForErrCompletion.Done()
	}()

	runJobsUntilAllDone(ctx, jobs, errCh)

	close(errCh)
	waitForErrCompletion.Wait()

	return result
}

// RunUntilFirstCompletion runs jobs concurrently
func RunUntilFirstCompletion(jobs []ConcurrentJob) error {
	var result error

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 10)
	waitForErrCompletion := sync.WaitGroup{}
	waitForErrCompletion.Add(1)
	go func() {
		result = monitorErrChForFirstError(cancel, errCh)
		waitForErrCompletion.Done()
	}()

	runJobsUntilAtleastOneDone(ctx, cancel, jobs, errCh)

	close(errCh)
	waitForErrCompletion.Wait()

	return result
}

func monitorErrChForFirstError(cancel context.CancelFunc, errCh chan error) error {
	var result error
	for err := range errCh {
		if result != nil {
			continue
		}
		result = err
		cancel()
	}
	return result
}

func runJobsUntilAllDone(ctx context.Context, jobs []ConcurrentJob, errCh chan error) {
	wg := sync.WaitGroup{}
	for _, job := range jobs {
		wg.Add(1)
		go func(job ConcurrentJob) {
			job.Start(ctx, errCh)
			wg.Done()
		}(job)
	}
	wg.Wait()
}

func runJobsUntilAtleastOneDone(ctx context.Context, cancel context.CancelFunc, jobs []ConcurrentJob, errCh chan error) {
	wg := sync.WaitGroup{}
	for _, job := range jobs {
		wg.Add(1)
		go func(job ConcurrentJob) {
			job.Start(ctx, errCh)
			cancel()
			wg.Done()
		}(job)
	}
	wg.Wait()
}
