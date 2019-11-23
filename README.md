# nursery
Structured Concurrency in Go

# Background
[Notes on structured concurrency, or: Go statement considered harmful](https://vorpus.org/blog/notes-on-structured-concurrency-or-go-statement-considered-harmful/#nurseries-a-structured-replacement-for-go-statements) is an article that compares the dangers of goto with the go statement.

While I don't necessarily agree with the content I can appreciate that even with Go's high-level abstraction of concurrency using Goroutines, Channels & the select statement it is possible to end up with deadlocks, leaked goroutines and race conditions.

Yet, implementing a higher-level abstraction for the simple use-cases mentioned is very straightforward in Go and this trivially simple package provides just that.

It exposes a function `RunConcurrently(jobs []ConcurrentJob) error` that takes an array of ConcurrentJob types and runs them concurrently ensuring that all jobs are completed before the call terminates. If all jobs terminate cleanly error is nil; otherwise the first non-nil error is returned.

In addition we also have `RunUntilFirstCompletion(jobs []ConcurrentJob) error` that takes an array of ConcurrentJob types and runs them concurrently but terminates after the completion of the earliest completing job. The beauty is that despite the early termination it blocks until all jobs have terminated (ie. released any used resources). If all jobs terminate cleanly error is nil; otherwise the first non-nil error is returned.

`ConcurrentJob` is a type that implements one function `Start` that takes a context and error channel as input. This is flipped into a helper function `ConcurrentJobFunc` that takes the same parameters but allows one to simply define a function as an input to `RunConcurrently`.

The only heed we need to pay when working with these functions is to ensure that within `ConcurrentJob` the context is checked for cancellation (`<-ctx.Done()`); and if received we exit the function releasing any resources we might have used.
