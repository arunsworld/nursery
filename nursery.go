// Package nursery implements "structured concurrency" in Go.
//
// It's based on this blog post: https://vorpus.org/blog/notes-on-structured-concurrency-or-go-statement-considered-harmful/
package nursery

import (
	"context"
	"sync"
	"time"
)

// ConcurrentJob contains procedural code that can run concurrently to another.
// Please ensure that you're listening to `context.Done()` - at which point you're required to clean up and exit.
// Publish any errors into the error channel but note that only the first error across the jobs will be returned.
// Finally ensure that you're not unsafely modifying shared state without protection and using go's built in
// channels for communicating rather than sharing memory.
type ConcurrentJob func(context.Context, chan error)

// RunConcurrentlyWithContext runs jobs concurrently until all jobs have either finished or any one job encountered an error.
// It wraps the parent context - so if the parent context is Done the jobs get the signal to wrap up
func RunConcurrentlyWithContext(parentCtx context.Context, jobs ...ConcurrentJob) error {
	var result error

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	errCh := make(chan error, 10)
	waitForErrCompletion := sync.WaitGroup{}
	waitForErrCompletion.Add(1)
	go func() {
		result = cancelOnFirstError(cancel, errCh)
		waitForErrCompletion.Done()
	}()

	runJobsUntilAllDone(ctx, jobs, errCh)

	close(errCh)
	waitForErrCompletion.Wait()

	return result
}

// RunConcurrently runs jobs concurrently until all jobs have either finished or any one job encountered an error.
func RunConcurrently(jobs ...ConcurrentJob) error {
	return RunConcurrentlyWithContext(context.Background(), jobs...)
}

// JobID is the key used to identify the JobID from the context for jobs running in copies
const JobID = jobIDKey("id")

type jobIDKey string

// RunMultipleCopiesConcurrentlyWithContext runs multiple copies of the given job until they have all finished or any
// one has encountered an error. The passed context can be optionally checked for an int value with key JobID counting up from 0
// to identify uniquely the copy that is run.
// It wraps the parent context - so if the parent context is Done the jobs get the signal to wrap up
func RunMultipleCopiesConcurrentlyWithContext(ctx context.Context, copies int, job ConcurrentJob) error {
	jobs := make([]ConcurrentJob, 0, copies)
	for i := 0; i < copies; i++ {
		idOfCopy := i
		jobs = append(jobs, func(ctx context.Context, errCh chan error) {
			ctx = context.WithValue(ctx, JobID, idOfCopy)
			job(ctx, errCh)
		})
	}
	return RunConcurrentlyWithContext(ctx, jobs...)
}

// RunMultipleCopiesConcurrently runs multiple copies of the given job until they have all finished or any
// one has encountered an error. The passed context can be optionally checked for an int value with key JobID counting up from 0
// to identify uniquely the copy that is run.
func RunMultipleCopiesConcurrently(copies int, job ConcurrentJob) error {
	return RunMultipleCopiesConcurrentlyWithContext(context.Background(), copies, job)
}

// RunUntilFirstCompletion runs jobs concurrently until atleast one job has finished or any job has encountered an error.
func RunUntilFirstCompletion(jobs ...ConcurrentJob) error {
	return RunUntilFirstCompletionWithContext(context.Background(), jobs...)
}

// RunUntilFirstCompletionWithContext runs jobs concurrently until atleast one job has finished or any job has encountered an error.
func RunUntilFirstCompletionWithContext(parentCtx context.Context, jobs ...ConcurrentJob) error {
	var result error

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	errCh := make(chan error, 10)
	waitForErrCompletion := sync.WaitGroup{}
	waitForErrCompletion.Add(1)
	go func() {
		result = cancelOnFirstError(cancel, errCh)
		waitForErrCompletion.Done()
	}()

	runJobsUntilAtleastOneDone(ctx, cancel, jobs, errCh)

	close(errCh)
	waitForErrCompletion.Wait()

	return result
}

// RunConcurrentlyWithTimeout runs jobs concurrently until all jobs have either finished or any one job encountered an error.
// or the timeout has expired
func RunConcurrentlyWithTimeout(timeout time.Duration, jobs ...ConcurrentJob) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return RunConcurrentlyWithContext(ctx, jobs...)
}

// RunUntilFirstCompletionWithTimeout runs jobs concurrently until atleast one job has finished or any job has encountered an error
// or the timeout has expired.
func RunUntilFirstCompletionWithTimeout(timeout time.Duration, jobs ...ConcurrentJob) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return RunUntilFirstCompletionWithContext(ctx, jobs...)
}

// IsContextDone is a utility function to check if the context is Done/Cancelled.
func IsContextDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func cancelOnFirstError(cancel context.CancelFunc, errCh chan error) error {
	err := <-errCh
	if err == nil {
		return nil
	}
	cancel()
	// drain the errCh so we don't block producers
	for range errCh {
	}
	return err
}

func runJobsUntilAllDone(ctx context.Context, jobs []ConcurrentJob, errCh chan error) {
	wg := sync.WaitGroup{}
	for _, job := range jobs {
		wg.Add(1)
		go func(job ConcurrentJob) {
			job(ctx, errCh)
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
			job(ctx, errCh)
			cancel()
			wg.Done()
		}(job)
	}
	wg.Wait()
}
