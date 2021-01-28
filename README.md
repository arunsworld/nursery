# nursery: structured concurrency in Go
[![GoDoc](https://godoc.org/github.com/arunsworld/nursery?status.svg)](https://godoc.org/github.com/arunsworld/nursery)
[![GoReportCard](https://goreportcard.com/badge/github.com/arunsworld/nursery)](https://goreportcard.com/badge/github.com/arunsworld/nursery)
[![CircleCI](https://circleci.com/gh/arunsworld/nursery.svg?style=svg)](https://circleci.com/gh/arunsworld/nursery)
<a href='https://github.com/jpoles1/gopherbadger' target='_blank'>![gopherbadger-tag-do-not-edit](https://img.shields.io/badge/Go%20Coverage-100%25-brightgreen.svg?longCache=true&style=flat)</a>

```go
RunConcurrently(
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
```

## Installation
```bash
go get -u github.com/arunsworld/nursery
```

[Notes on structured concurrency, or: Go statement considered harmful](https://vorpus.org/blog/notes-on-structured-concurrency-or-go-statement-considered-harmful/#nurseries-a-structured-replacement-for-go-statements) is an article that compares the dangers of goto with the go statement.

While I don't necessarily agree with the entire content I can appreciate that even with Go's high-level abstraction of concurrency using Goroutines, Channels & the select statement it is possible to end up with unreadable code, deadlocks, leaked goroutines, race conditions and poor error handling.

Implementing a higher-level abstraction for the use-cases mentioned is very straightforward in Go and this simple package provides just that.

The following functions are provided:
* `RunConcurrently(jobs ...ConcurrentJob) error`: takes an array of `ConcurrentJob`s and runs them concurrently ensuring that all jobs are completed before the call terminates. If all jobs terminate cleanly error is nil; otherwise the first non-nil error is returned.
* `RunConcurrentlyWithContext(parentCtx context.Context, jobs ...ConcurrentJob) error`: is the RunConcurrently behavior but additionally wraps a context that's passed in allowing cancellations of the parentCtx to get propagated.
* `RunMultipleCopiesConcurrently(copies int, job ConcurrentJob) error`: makes copies of the given job and runs them concurrently. This is useful for cases where we want to execute multiple slow consumers taking jobs from a channel until the job is finished. The channel itself can be fed by a producer that is run concurrently with the job running the consumers. Each job's context is also passed an unique index with key `nursery.JobID` - a 0 based int - that maybe used as a job identity if required.
* `RunMultipleCopiesConcurrentlyWithContext(ctx context.Context, copies int, job ConcurrentJob) error`: is the RunMultipleCopiesConcurrently behavior with a context that allows cancellation to be propagated to the jobs.
* `RunUntilFirstCompletion(jobs ...ConcurrentJob) error`: takes an array of `ConcurrentJob`s and runs them concurrently but terminates after the completion of the earliest completing job. A key point here is that despite early termination it blocks until all jobs have terminated (ie. released any used resources). If all jobs terminate cleanly error is nil; otherwise the first non-nil error is returned.
* `RunUntilFirstCompletionWithContext(parentCtx context.Context, jobs ...ConcurrentJob) error`: is the RunUntilFirstCompletion behavior but additionally wraps a context that's passed in allowing cancellations of the parentCtx to get propagated.
* `RunConcurrentlyWithTimeout(timeout time.Duration, jobs ...ConcurrentJob) error`: is similar in behavior to `RunConcurrently` except it also takes a timeout and can cause the function to terminate earlier if timeout has expired. As before we wait for all jobs to have cleanly terminated.
* `RunUntilFirstCompletionWithTimeout(timeout time.Duration, jobs ...ConcurrentJob) error`: is similar in behavior to `RunUntilFirstCompletion` with an additional timeout clause.

`ConcurrentJob` is a simple function that takes a context and error channel. We need to ensure that we're listening to the `Done()` channel on context and if invoked to clean-up resources and bail out. Errors are to be published to the error channel for proper handling.

Note: while this package simplifies the semantics of defining and executing concurrent code it cannot protect against bad concurrent programming such as using shared resources across jobs leading to data corruption or panics due to race conditions.

You may also be interested in reading [Structured Concurrency in Go](https://medium.com/@arunsworld/structured-concurrency-in-go-b800c7c4434e).

The library includes a utility function: `IsContextDone(context.Context)` to check if the passed in context is done or not. This can be used as a guard clause in a for loop within a ConcurrentJob using the passed in context to decide whether to stop processing and return or continue.
