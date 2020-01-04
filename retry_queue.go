package jobkit

import (
	"context"
	"runtime"
	"time"

	"github.com/blend/go-sdk/async"
	"github.com/blend/go-sdk/ex"
	"github.com/blend/go-sdk/logger"
)

// NewRetryQueue returns a new retry queue.
func NewRetryQueue(action async.WorkAction, options ...RetryQueueOption) *RetryQueue {
	rq := &RetryQueue{
		Latch:       async.NewLatch(),
		Parallelism: runtime.NumCPU(),
		Action:      action,
		Work:        make(chan *RetryQueueWorkItem, 32),
	}
	for _, opt := range options {
		opt(rq)
	}
	return rq
}

// RetryQueueOption is an option or mutator for a retry queue.
type RetryQueueOption func(*RetryQueue)

// OptRetryQueueMaxAttempts sets the retry queue max attempts.
func OptRetryQueueMaxAttempts(maxAttempts int) RetryQueueOption {
	return func(rq *RetryQueue) {
		rq.MaxAttempts = maxAttempts
	}
}

// OptRetryQueueRetryWait sets the retry wait to a const value.
func OptRetryQueueRetryWait(wait time.Duration) RetryQueueOption {
	return func(rq *RetryQueue) {
		rq.RetryWaitProvider = func(_ *RetryQueueWorkItem) time.Duration {
			return wait
		}
	}
}

// OptRetryQueueRetryWaitProvider sets the retry wait provider.
func OptRetryQueueRetryWaitProvider(waitProvider func(*RetryQueueWorkItem) time.Duration) RetryQueueOption {
	return func(rq *RetryQueue) {
		rq.RetryWaitProvider = waitProvider
	}
}

// OptRetryQueueRetryWaitBackoff sets the retry wait to be a backoff based on a base and
// the number of attempts.
func OptRetryQueueRetryWaitBackoff(base time.Duration) RetryQueueOption {
	return func(rq *RetryQueue) {
		rq.RetryWaitProvider = func(rqwi *RetryQueueWorkItem) time.Duration {
			return time.Duration(rqwi.Attempts) * base
		}
	}
}

// RetryQueue is a queue that retries on error.
type RetryQueue struct {
	Latch             *async.Latch
	Log               logger.Log
	Work              chan *RetryQueueWorkItem
	Parallelism       int
	MaxAttempts       int
	RetryWaitProvider func(*RetryQueueWorkItem) time.Duration
	Action            async.WorkAction
}

// Add adds an item to the queue.
func (rq *RetryQueue) Add(ctx context.Context, item interface{}) {
	rq.Work <- &RetryQueueWorkItem{
		Context: ctx,
		Item:    item,
	}
}

// Start starts the retry queue.
func (rq *RetryQueue) Start() error {
	if !rq.Latch.CanStart() {
		return async.ErrCannotStart
	}
	rq.Latch.Started()

	var workItem *RetryQueueWorkItem
	workers := make([]*RetryQueueWorker, rq.Parallelism)
	var current int
	for x := 0; x < rq.Parallelism; x++ {
		workers[x] = &RetryQueueWorker{
			Latch:          async.NewLatch(),
			Work:           make(chan *RetryQueueWorkItem),
			Action:         rq.Action,
			OnErrorHandler: rq.onError,
		}
		go workers[x].Start()
		<-workers[x].NotifyStarted()
	}

	for {
		select {
		case workItem = <-rq.Work:
			workers[current].Work <- workItem
			current = (current + 1) % rq.Parallelism
		case <-rq.Latch.NotifyStopping():
			for x := 0; x < rq.Parallelism; x++ {
				workers[x].Stop()
			}
			rq.Latch.Stopped()
			return nil
		}
	}
}

// Stop stops the retry queue.
func (rq *RetryQueue) Stop() error {
	if !rq.Latch.CanStop() {
		return async.ErrCannotStop
	}
	rq.Latch.Stopping()
	<-rq.Latch.NotifyStopped()
	return nil
}

// NotifyStarted returns the started notification channel.
func (rq *RetryQueue) NotifyStarted() <-chan struct{} {
	return rq.Latch.NotifyStarted()
}

// NotifyStopped returns the stopped notification channel.
func (rq *RetryQueue) NotifyStopped() <-chan struct{} {
	return rq.Latch.NotifyStopped()
}

func (rq *RetryQueue) onError(wi *RetryQueueWorkItem, err error) {
	logger.MaybeError(rq.Log, err)

	// inrecement attempts
	wi.Attempts = wi.Attempts + 1

	if rq.RetryWaitProvider != nil {
		if wait := rq.RetryWaitProvider(wi); wait > 0 {
			logger.MaybeDebugf(rq.Log, "retry queue; work item error; waiting %v", wait)
			select {
			case <-time.After(wait):
				break
			case <-rq.Latch.NotifyStopping():
				return
			}
		}
	}
	if rq.MaxAttempts == 0 || wi.Attempts < rq.MaxAttempts {
		if rq.MaxAttempts > 0 {
			logger.MaybeDebugf(rq.Log, "retry queue; work item error; requeueing (%d of %d)", wi.Attempts, rq.MaxAttempts)
		} else {
			logger.MaybeDebugf(rq.Log, "retry queue; work item error; requeueing (%d of inf)", wi.Attempts)
		}
		rq.Work <- wi
	}
}

// RetryQueueWorker is a background worker for a retry queue.
type RetryQueueWorker struct {
	Latch  *async.Latch
	Action async.WorkAction
	Work   chan *RetryQueueWorkItem

	OnStartHandler    func(*RetryQueueWorkItem)
	OnCompleteHandler func(*RetryQueueWorkItem)
	OnErrorHandler    func(*RetryQueueWorkItem, error)
}

// Start starts the retry queue worker.
func (rqw *RetryQueueWorker) Start() error {
	rqw.Latch.Started()
	var workItem *RetryQueueWorkItem
	for {
		select {
		case workItem = <-rqw.Work:
			rqw.Execute(workItem)
		case <-rqw.Latch.NotifyStopping():
			rqw.Latch.Stopped()
			return nil
		}
	}
}

// Stop stops the retry queue worker.
func (rqw *RetryQueueWorker) Stop() error {
	if !rqw.Latch.CanStop() {
		return async.ErrCannotStop
	}
	rqw.Latch.Stopping()
	<-rqw.Latch.NotifyStopped()
	return nil
}

// NotifyStarted returns the started notification channel.
func (rqw *RetryQueueWorker) NotifyStarted() <-chan struct{} {
	return rqw.Latch.NotifyStarted()
}

// NotifyStopped returns the stopped notification channel.
func (rqw *RetryQueueWorker) NotifyStopped() <-chan struct{} {
	return rqw.Latch.NotifyStopped()
}

// Execute handles a work item.
func (rqw *RetryQueueWorker) Execute(workItem *RetryQueueWorkItem) {
	defer func() {
		if r := recover(); r != nil {
			rqw.OnError(workItem, ex.New(r))
		}
	}()

	rqw.OnStart(workItem)
	if err := rqw.Action(workItem.Context, workItem.Item); err != nil {
		rqw.OnError(workItem, err)
		return
	}
	rqw.OnComplete(workItem)
}

// OnStart handles on start steps.
func (rqw *RetryQueueWorker) OnStart(wi *RetryQueueWorkItem) {
	if rqw.OnStartHandler != nil {
		rqw.OnStartHandler(wi)
	}
}

// OnError handles on error steps.
func (rqw *RetryQueueWorker) OnError(wi *RetryQueueWorkItem, err error) {
	if rqw.OnErrorHandler != nil {
		rqw.OnErrorHandler(wi, err)
	}
}

// OnComplete handles on complete steps.
func (rqw *RetryQueueWorker) OnComplete(wi *RetryQueueWorkItem) {
	if rqw.OnCompleteHandler != nil {
		rqw.OnCompleteHandler(wi)
	}
}

// RetryQueueWorkItem is a work item for the retry queue.
type RetryQueueWorkItem struct {
	Context  context.Context
	Item     interface{}
	Attempts int
}
