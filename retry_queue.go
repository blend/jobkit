package jobkit

import (
	"context"
	"time"

	"github.com/blend/go-sdk/async"
)

// NewRetryQueue returns a new retry queue.
func NewRetryQueue(action async.WorkAction, options ...RetryQueueOption) *RetryQueue {
	rq := &RetryQueue{
		Latch:  async.NewLatch(),
		Action: action,
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

// OptRetryQueueConstRetryWait sets the retry wait to a const value.
func OptRetryQueueConstRetryWait(wait time.Duration) RetryQueueOption {
	return func(rq *RetryQueue) {
		rq.RetryWaitProvider = func(_ *RetryWorkItem) time.Duration {
			return wait
		}
	}
}

// RetryQueue is a queue that retries on error.
type RetryQueue struct {
	Latch             *async.Latch
	Work              chan *RetryWorkItem
	MaxAttempts       int
	RetryWaitProvider func(*RetryWorkItem) time.Duration
	Action            async.WorkAction
}

// RetryWorkItem is a work item for the retry queue.
type RetryWorkItem struct {
	Context  context.Context
	Item     interface{}
	Attempts int
}

// Add adds an item to the queue.
func (rq *RetryQueue) Add(ctx context.Context, item interface{}) {
	rq.Work <- &RetryWorkItem{
		Context: ctx,
		Item:    item,
	}
}

// Start starts the retry queue.
func (rq *RetryQueue) Start() error {
	if !rq.Latch.CanStart() {
		return async.ErrCannotStart
	}

	var workItem *RetryWorkItem
	for {
		select {
		case workItem = <-rq.Work:
			go rq.recover(workItem)
		case <-rq.Latch.NotifyStopping():
			rq.Latch.Stopped()
			return nil
		}
	}

	return nil
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

func (rq *RetryQueue) recover(workItem *RetryWorkItem) {

}
