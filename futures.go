package future

import (
	"context"
	"fmt"
	"reflect"
)

// poolKeyType is a private type used as a key for storing a FutureThreadPool
// in a context. It ensures type safety when associating a thread pool with a
// context.
type poolKeyType struct{}

// poolKey is the key used to store and retrieve a FutureThreadPool in a
// context.
var poolKey = poolKeyType{}

// FutureThreadPool manages a pool of goroutines to limit concurrency for
// Future-based computations.
//
// It uses a semaphore channel to restrict the number of concurrent operations,
// ensuring that no more than the specified maximum number of Futures execute
// simultaneously. This is useful for controlling resource usage in
// asynchronous workflows.
type FutureThreadPool struct {
	sem chan struct{}
}

// NewPool creates a new FutureThreadPool with a specified number of threads.
func NewPool(maxThreads int) *FutureThreadPool {
	return &FutureThreadPool{sem: make(chan struct{}, maxThreads)}
}

// WithPool associates a FutureThreadPool with a context, enabling Futures to
// use the pool for concurrency control when executed with
// NewFutureWithContext.
//
// The pool is stored in the context using a private key, allowing it to be
// retrieved later by the future.
func WithPool(ctx context.Context, pool *FutureThreadPool) context.Context {
	return context.WithValue(ctx, poolKey, pool)
}

// poolFromContext retrieves the FutureThreadPool stored in the provided
// context, if any.
//
// It is used internally by NewFutureWithContext to access the thread pool for
// concurrency management. If no pool is found, it returns nil, allowing the
// Future to run without concurrency restrictions.
func poolFromContext(ctx context.Context) *FutureThreadPool {
	p, _ := ctx.Value(poolKey).(*FutureThreadPool)
	return p
}

// acquire reserves a slot in the FutureThreadPool's semaphore, blocking if the
// maximum concurrency limit has been reached.
//
// This is called internally by NewFutureWithContext to ensure that the number
// of concurrent Futures does not exceed the pool's limit.
func (p *FutureThreadPool) acquire() { p.sem <- struct{}{} }

// release frees a slot in the FutureThreadPool's semaphore, allowing another
// goroutine to proceed.
//
// This is called internally by NewFutureWithContext after a Future's
// computation completes, ensuring proper resource cleanup.
func (p *FutureThreadPool) release() { <-p.sem }

// Future represents an asynchronous computation that will eventually produce
// a value of type T or an error. Providing a way to track the completion of
// a task and retrieve its result when ready.
type Future[T any] struct {
	done chan struct{}
	res  T
	err  error
}

// NewFuture creates a new Future that executes the provided function
// asynchronously.
//
// The function fn is run in a separate goroutine, and the Future is marked as
// done when the computation completes, whether successfully or with an error.
// This is typically used to initiate asynchronous tasks whose results can be
// retrieved later.
func NewFuture[T any](fn func() (T, error)) *Future[T] {
	f := &Future[T]{done: make(chan struct{})}

	go runFuture(fn, f)

	return f
}

// Done returns a channel that is closed when the Future's computation
// completes.
//
// It allows waiting for or polling the completion of the asynchronous task.
// Typically used with a select statement or to block until the Future is
// resolved.
func (f *Future[T]) Done() <-chan struct{} { return f.done }

// Get retrieves the result and error of the Future's computation.
//
// It blocks until the computation is complete, as indicated by Done(), and
// then returns the result and any error from the asynchronous task. Use this
// to access the outcome of the Future after it has resolved.
func (f *Future[T]) Get() (T, error) {
	<-f.Done()
	return f.res, f.err
}

// NewFutureWithContext is identical to NewFuture but executes a future with a
// given context.
//
// The function WithPool can be used to specify the maximum number of
// concurrent futures that can run at the same time within a context. Useful
// for limiting the amount of cpu usage or amount of concurrent web requests.
func NewFutureWithContext[T any](
	ctx context.Context, fn func() (T, error)) *Future[T] {
	f := &Future[T]{done: make(chan struct{})}

	go runFutureWithContext(ctx, fn, f)

	return f
}

// runFuture executes the given function and stores the results in the passed
// future.
func runFuture[T any](fn func() (T, error), f *Future[T]) {
	defer func() {
		if rec := recover(); rec != nil {
			f.err = fmt.Errorf("panic in future: %v", rec)
		}
		close(f.done)
	}()
	f.res, f.err = fn()
}

// runFutureWithContext is identical to runFuture but executes a future with a
// context, typically for limiting the total number of concurrent workers.
func runFutureWithContext[T any](
	ctx context.Context, fn func() (T, error), f *Future[T]) {
	if pool := poolFromContext(ctx); pool != nil {
		pool.acquire()
		defer pool.release()
	}
	runFuture(fn, f)
}

// Then creates a new Future that applies the provided function to the result
// of an existing Future.
//
// It chains an asynchronous computation, executing fn with the result of the
// input Future f once f completes, provided f did not produce an error. If f
// fails, the new Future propagates the error without invoking fn. This is
// useful for composing asynchronous operations in a sequence.
func Then[T1, T2 any](f *Future[T1], fn func(T1) (T2, error)) *Future[T2] {
	newFutureFn := func() (T2, error) {
		res, err := f.Get()
		if err != nil {
			var newRes T2
			return newRes, err
		}
		return fn(res)
	}

	return NewFuture(newFutureFn)
}

// AsCompleted creates an iterator that yields completed Futures in the order
// they resolve.
//
// It is designed for use with Go's range syntax for easy usage. The iterator
// terminates when all Futures are resolved.
func AsCompleted[T any](futures ...*Future[T]) func(func(*Future[T]) bool) {
	futureMap := make(map[<-chan struct{}]*Future[T])
	cases := make([]reflect.SelectCase, len(futures))
	for i, f := range futures {
		doneChan := f.Done()
		cases[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(doneChan),
		}
		futureMap[doneChan] = f
	}

	return func(yield func(*Future[T]) bool) {
		for len(cases) > 0 {
			chosen, _, _ := reflect.Select(cases)
			f := futureMap[cases[chosen].Chan.Interface().(<-chan struct{})]
			if !yield(f) {
				return
			}
			cases = append(cases[:chosen], cases[chosen+1:]...)
		}
	}
}

// WaitForAll blocks until all provided Futures have completed.
//
// It blocks till all futures complete, whether they succeeded or failed. This
// is useful for synchronizing multiple asynchronous tasks before proceeding
// with further processing.
func WaitForAll[T any](futures ...*Future[T]) {
	for _, f := range futures {
		<-f.Done()
	}
}
