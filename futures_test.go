package future_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	future "github.com/GreatValueCreamSoda/go-futures"
)

// TestNewFuture tests the basic functionality of NewFuture.
func TestNewFuture(t *testing.T) {
	// Test successful computation
	f := future.NewFuture(func() (int, error) {
		return 42, nil
	})

	res, err := f.Get()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if res != 42 {
		t.Errorf("Expected result 42, got %v", res)
	}

	// Test error case
	f = future.NewFuture(func() (int, error) {
		return 0, errors.New("test error")
	})

	res, err = f.Get()
	if err == nil || err.Error() != "test error" {
		t.Errorf("Expected error 'test error', got %v", err)
	}
	if res != 0 {
		t.Errorf("Expected zero result, got %v", res)
	}

	// Test panic recovery
	f = future.NewFuture(func() (int, error) {
		panic("test panic")
	})

	res, err = f.Get()
	if err == nil || err.Error() != "panic in future: test panic" {
		t.Errorf("Expected panic error, got %v", err)
	}
	if res != 0 {
		t.Errorf("Expected zero result, got %v", res)
	}
}

// TestNewFutureWithContext tests NewFutureWithContext with and without a thread pool.
func TestNewFutureWithContext(t *testing.T) {
	// Test without pool
	ctx := context.Background()
	f := future.NewFutureWithContext(ctx, func() (string, error) {
		return "hello", nil
	})

	res, err := f.Get()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if res != "hello" {
		t.Errorf("Expected result 'hello', got %v", res)
	}

	// Test with pool
	pool := future.NewPool(1)
	ctx = future.WithPool(ctx, pool)

	var counter int32
	f1 := future.NewFutureWithContext(ctx, func() (int, error) {
		atomic.AddInt32(&counter, 1)
		time.Sleep(200 * time.Millisecond)
		return 1, nil
	})
	time.Sleep(10 * time.Millisecond) // Small delay to ensure f1 starts
	f2 := future.NewFutureWithContext(ctx, func() (int, error) {
		if atomic.LoadInt32(&counter) != 1 {
			t.Errorf("Expected only one goroutine running, got %v", atomic.LoadInt32(&counter))
		}
		return 2, nil
	})

	res1, err1 := f1.Get()
	res2, err2 := f2.Get()
	if err1 != nil || err2 != nil {
		t.Errorf("Expected no errors, got %v, %v", err1, err2)
	}
	if res1 != 1 || res2 != 2 {
		t.Errorf("Expected results 1, 2, got %v, %v", res1, res2)
	}
}

// TestFutureThreadPool tests the concurrency limiting of FutureThreadPool.
func TestFutureThreadPool(t *testing.T) {
	pool := future.NewPool(2)
	ctx := future.WithPool(context.Background(), pool)

	var active int32
	var maxConcurrent int32
	var mu sync.Mutex
	f := make([]*future.Future[int], 5)

	for i := 0; i < 5; i++ {
		f[i] = future.NewFutureWithContext(ctx, func() (int, error) {
			mu.Lock()
			current := atomic.AddInt32(&active, 1)
			if current > maxConcurrent {
				maxConcurrent = current
			}
			mu.Unlock()
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&active, -1)
			return i, nil
		})
	}

	future.WaitForAll(f...)
	if maxConcurrent > 2 {
		t.Errorf("Expected max 2 concurrent goroutines, got %v", maxConcurrent)
	}

	for i, f := range f {
		res, err := f.Get()
		if err != nil {
			t.Errorf("Expected no error for future %d, got %v", i, err)
		}
		if res != i {
			t.Errorf("Expected result %d, got %v", i, res)
		}
	}
}

// TestThen tests the Then function for chaining Futures.
func TestThen(t *testing.T) {
	// Test successful chaining
	f1 := future.NewFuture(func() (int, error) {
		return 10, nil
	})
	f2 := future.Then(f1, func(x int) (string, error) {
		return fmt.Sprintf("value: %d", x), nil
	})

	res, err := f2.Get()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if res != "value: 10" {
		t.Errorf("Expected result 'value: 10', got %v", res)
	}

	// Test error propagation
	f1 = future.NewFuture(func() (int, error) {
		return 0, errors.New("first error")
	})
	f2 = future.Then(f1, func(x int) (string, error) {
		t.Error("Chained function should not be called on error")
		return "", nil
	})

	res, err = f2.Get()
	if err == nil || err.Error() != "first error" {
		t.Errorf("Expected error 'first error', got %v", err)
	}
	if res != "" {
		t.Errorf("Expected empty string, got %v", res)
	}
}

// TestAsCompleted tests the AsCompleted iterator.
func TestAsCompleted(t *testing.T) {
	f := make([]*future.Future[int], 3)

	f[0] = future.NewFuture(func() (int, error) {
		time.Sleep(300 * time.Millisecond)
		return 1, nil
	})
	time.Sleep(10 * time.Millisecond) // Small delay to ensure f0 starts
	f[1] = future.NewFuture(func() (int, error) {
		time.Sleep(100 * time.Millisecond)
		return 2, nil
	})
	time.Sleep(10 * time.Millisecond) // Small delay to ensure f1 starts
	f[2] = future.NewFuture(func() (int, error) {
		time.Sleep(200 * time.Millisecond)
		return 3, nil
	})

	var results []int
	for f := range future.AsCompleted(f...) {
		res, err := f.Get()
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		results = append(results, res)
	}

	expected := []int{2, 3, 1} // Ordered by completion time
	if !reflect.DeepEqual(results, expected) {
		t.Errorf("Expected results %v, got %v", expected, results)
	}
}

// TestWaitForAll tests the WaitForAll function.
func TestWaitForAll(t *testing.T) {
	f := []*future.Future[int]{
		future.NewFuture(func() (int, error) {
			time.Sleep(10 * time.Millisecond)
			return 1, nil
		}),
		future.NewFuture(func() (int, error) {
			return 0, errors.New("test error")
		}),
	}

	start := time.Now()
	future.WaitForAll(f...)
	duration := time.Since(start)

	if duration < 10*time.Millisecond {
		t.Errorf("WaitForAll returned too early, duration: %v", duration)
	}

	res1, err1 := f[0].Get()
	if err1 != nil || res1 != 1 {
		t.Errorf("Expected result 1 with no error, got %v, %v", res1, err1)
	}

	res2, err2 := f[1].Get()
	if err2 == nil || err2.Error() != "test error" {
		t.Errorf("Expected error 'test error', got %v", err2)
	}
	if res2 != 0 {
		t.Errorf("Expected result 0, got %v", res2)
	}
}
