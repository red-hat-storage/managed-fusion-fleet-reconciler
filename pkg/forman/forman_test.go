package forman

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// testPanic tests if the work is done after simulating panic for a given
// number of times
func testPanic() error {
	count := 0
	panicTillCount := 3
	workDone := make(chan bool)

	reconcile := func(req Request) Result {
		if count < panicTillCount {
			count++
			panic("Simulated panic")
		}
		workDone <- true
		return Result{}
	}

	requests := GoForman(zap.NewNop(), 1, reconcile)
	requests <- Request{Name: "panic"}

	<-workDone
	if count != panicTillCount {
		return fmt.Errorf("testPanic: Expected count to be %d, got %d", panicTillCount, count)
	}

	return nil
}

// testRequeue tests if the work is requeued for a given number of times and
// then work is completed.
func testRequeue() error {
	count := 0
	workDone := make(chan bool)
	reqeueTillCount := 3

	reconcile := func(req Request) Result {
		if count < reqeueTillCount {
			count++
			return Result{Requeue: true, After: 1}
		}
		workDone <- true
		return Result{Requeue: false}
	}

	requests := GoForman(zap.NewNop(), 2, reconcile)
	requests <- Request{Name: "requeue"}

	<-workDone
	if count != reqeueTillCount {
		return fmt.Errorf("testRequeue: Expected count to be %d, got %d", reqeueTillCount, count)
	}

	return nil
}

// testWorkDone tests if the work is done after a successful reconcile
func testWorkDone() error {
	workDone := make(chan bool)

	reconcile := func(req Request) Result {
		workDone <- true
		return Result{Requeue: false}
	}

	requests := GoForman(zap.NewNop(), 1, reconcile)
	requests <- Request{Name: "workDone"}

	// Start the timer for 5 second
	timer := time.NewTimer(5 * time.Second)
	select {
	case <-workDone:
	// The work is not done and the timer expired, the test should fail
	case <-timer.C:
		return fmt.Errorf("testWorkDone:test timed out after 1 second")
	}

	return nil
}

// testMultipleRequestForSameProvider tests if multiple requests for the same
// provider are handled by a single worker but not by multiple workers at the same time
func testMultipleRequestForSameProvider() error {
	var err error
	workGroup := sync.WaitGroup{}
	workDoneByOneWorker := atomic.Bool{}
	workGroup.Add(2)

	reconcile := func(req Request) Result {
		defer workGroup.Done()
		if ok := workDoneByOneWorker.Load(); ok {
			err = fmt.Errorf("testMultipleRequestForSameProvider: Expected work to be done by one worker, but got done by multiple workers")
		}
		workDoneByOneWorker.Store(true)
		time.Sleep(1 * time.Second)
		workDoneByOneWorker.Store(false)
		return Result{Requeue: false}
	}

	requests := GoForman(zap.NewNop(), 2, reconcile)
	requests <- Request{Name: "workDone"}
	requests <- Request{Name: "workDone"}

	workGroup.Wait()
	return err
}

// testRequestForDifferentProvider tests if request for different providers are
// handled by different workers and same provider is not handled by multiple
// workers at the same time
func testRequestForDifferentProvider() error {
	totalRequests := 10
	var err error
	workGroup := sync.WaitGroup{}
	workGroup.Add(totalRequests)
	work := sync.Map{}
	reconcile := func(req Request) Result {
		defer workGroup.Done()
		val, ok := work.Load(req.Name)
		if !ok {
			val = 0
			work.Store(req.Name, val)
		}
		work.Store(req.Name, val.(int)+1)
		return Result{Requeue: false}
	}

	requests := GoForman(zap.NewNop(), 5, reconcile)

	for i := 0; i < totalRequests; i++ {
		requests <- Request{Name: fmt.Sprintf("work-%d", i)}
	}

	workGroup.Wait()
	// check map contains counter for all the requests and the counter is 1 not
	// more than that as the work is done by only one worker
	work.Range(func(key, value interface{}) bool {
		if value.(int) != 1 {
			err = fmt.Errorf("testRequestForDifferentProvider: Expected work to be done by one worker, but got done by multiple workers")
			return false
		}
		return true
	})

	return err
}

func TestGoForman(t *testing.T) {
	var allErrors []error

	if err := testWorkDone(); err != nil {
		allErrors = append(allErrors, err)
	}

	if err := testPanic(); err != nil {
		allErrors = append(allErrors, err)
	}

	if err := testRequeue(); err != nil {
		allErrors = append(allErrors, err)
	}

	if err := testMultipleRequestForSameProvider(); err != nil {
		allErrors = append(allErrors, err)
	}

	if err := testRequestForDifferentProvider(); err != nil {
		allErrors = append(allErrors, err)
	}

	assert.Empty(t, allErrors, "Expected no errors, but got %v error(s)", allErrors)
}
