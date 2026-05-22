# go-futures

[![Go Reference](https://pkg.go.dev/badge/github.com/GreatValueCreamSoda/go-futures.svg)](https://pkg.go.dev/github.com/GreatValueCreamSoda/go-futures)
[![Go Report Card](https://goreportcard.com/badge/github.com/GreatValueCreamSoda/go-futuresl)](https://goreportcard.com/report/github.com/GreatValueCreamSoda/go-futures)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A lightweight, generic Future and thread-pool control implementation for Go.
This package provides an easy way to execute asynchronous computations, chain them together, limit concurrency, coordinate results, and handle concurrent errors or panics.

## What Is A Future
A `Future[T]` represents a proxy for a value that isn't necessarily known yet.
Upon creation via `NewFuture`, a background goroutine is immediately spawned to execute the task.
The future is monitored via a data-less channel (`chan struct{}`). Calling `.Get()` blocks execution until this channel closes, safely delivering the typed result or error.


## Features

- **Generic Futures**: Run asynchronous computations with strongly typed results.
- **Thread Pool**: Limit concurrency via context-propagated semaphores.
- **Chaining & Recovery**: Use `Then` to compose operations and `Catch` for error recovery fallback pipelines.
- **`AsCompleted`**: Process results reactively in the exact order they resolve using Go range-ready iterators.
- **Synchronization**: Batch-wait for tasks with `WaitForAll` or aggregate values with `All`.
- **Asynchronous Mapping**: Spin up slices of futures instantly from raw slices using `Map` and `MapWithContext`.

### Panic Recovery
Asynchronous panics can easily bring down an entire Go runtime.
This library wraps all internal execution blocks with a deferred recovery function.
If a user-supplied function panics, the panic is intercepted, converted into a standard Go error, and delivered safely through the `.Get()` method without disrupting other running futures.

### Concurrency Limits via Context
Instead of forcing you to pass manual workers or channel queues around, concurrency limits are driven implicitly through Go's standard `context.Context`.
By attaching a `FutureThreadPool` to your context via `WithPool`, any downstream future initiated via `NewFutureWithContext` will automatically acquire and release slots on a channel-based semaphore before executing.

---

## Installation

```bash
go get github.com/GreatValueCreamSoda/go-futures
```

## Quick Start

- Running a simple future

```go
f := future.NewFuture(func() (int, error) {
    // simulate work
    time.Sleep(time.Second)
    return 42, nil
})

result, err := f.Get()
if err != nil {
    log.Fatal(err)
}
fmt.Println("Result:", result) // Output: 42
```

- Limiting concurrency with a pool

```go
pool := future.NewPool(2)
ctx := future.WithPool(context.Background(), pool)

tasks := []*future.Future[int]{}
for i := 0; i < 5; i++ {
    n := i
    tasks = append(tasks, future.NewFutureWithContext(ctx, func() (int, error) {
        fmt.Println("Running task", n)
        time.Sleep(time.Second)
        return n * n, nil
    }))
}

future.WaitForAll(tasks...)
fmt.Println("All tasks completed.")
```

- Chaining computations with `Then`

```go
f1 := future.NewFuture(func() (int, error) {
    return 10, nil
})

f2 := future.Then(f1, func(x int) (string, error) {
    return fmt.Sprintf("Value is %d", x), nil
})

res, err := f2.Get()
if err != nil {
    log.Fatal(err)
}
fmt.Println(res) // Output: Value is 10
```

- Error Recovery with `Catch`

```go
f1 := future.NewFuture(func() (int, error) {
    return 0, errors.New("something went wrong")
})

// Catch intercepts the error and provides a fallback value
f2 := future.Catch(f1, func(err error) (int, error) {
    fmt.Println("Recovered from error:", err)
    return 100, nil
})

res, _ := f2.Get()
fmt.Println("Fallback Result:", res) // Output: 100
```

- Processing tasks as they finish

```go
tasks := []*future.Future[int]{
    future.NewFuture(func() (int, error) { time.Sleep(time.Second); return 1, nil }),
    future.NewFuture(func() (int, error) { return 2, nil }),
    future.NewFuture(func() (int, error) { time.Sleep(2*time.Second); return 3, nil }),
}

iter := future.AsCompleted(tasks...)
iter(func(f *future.Future[int]) bool {
    res, _ := f.Get()
    fmt.Println("Completed:", res)
    return true
})
```

- Combining Futures with `All`

```go
f1 := future.NewFuture(func() (int, error) { return 10, nil })
f2 := future.NewFuture(func() (int, error) { return 20, nil })

combined := future.All(f1, f2)
results, err := combined.Get()
if err != nil {
    log.Fatal(err)
}
fmt.Println(results) // Output: [10, 20]
// Note: If ANY future fails, All fails immediately with that error.
```

- Asynchronous Mapping with `Map`

```go
items := []int{1, 2, 3, 4, 5}

// Automatically processes the items concurrently using futures
futures := future.Map(items, func(n int) (int, error) {
    return n * 2, nil
})

for _, f := range futures {
    res, _ := f.Get()
    fmt.Println(res)
}
```

## When to use

- When running several concurrent io bound tasks
- When tasks are hard to group together or are spread across multiple contexts
- When concurrent code has multiple branching paths

## When not to use

- When concurrent functions are short lived
- When concurrent patterns can be expressed easily with simple go routines and channels
- When keeping control flow simple
