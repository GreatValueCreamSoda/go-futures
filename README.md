# go-futures

[![Go Reference](https://pkg.go.dev/badge/github.com/GreatValueCreamSoda/go-futures.svg)](https://pkg.go.dev/github.com/GreatValueCreamSoda/go-futures)
[![Go Report Card](https://goreportcard.com/badge/github.com/GreatValueCreamSoda/go-futuresl)](https://goreportcard.com/report/github.com/GreatValueCreamSoda/go-futures)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A lightweight, generic Future and thread-pool control implementation for Go.
This package provides an easy way to execute asynchronous computations, chain them together, limit concurrency, coordinate results, and handel concurrent errors or panics.

## Features

- Generic Futures: Run asynchronous computations with typed results.
- Thread Pool: Limit concurrency using a pool of goroutines.
- Chaining: Use `Then` to compose operations.
- `AsCompleted`: Process results in order of completion.
- Synchronization: Wait for multiple tasks to finish with WaitForAll.
- Context Support: Control concurrency with NewFutureWithContext and WithPool.

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

## When to use

- When running several concurrent io bound tasks
- When tasks are hard to group together or are spread across multiple contexts
- When concurrent code has multiple branching paths

## When not to use

- When concurrent functions are short lived
- When concurrent patterns can be expressed easily with simple go routines and channels
- When keeping control flow simple
