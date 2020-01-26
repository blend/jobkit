package jobkit

import (
	"context"
	"fmt"
	"testing"

	"github.com/blend/go-sdk/assert"
)

func TestRetryQueue(t *testing.T) {
	assert := assert.New(t)

	attempts := 5
	done := make(chan struct{})

	rtq := NewRetryQueue(func(_ context.Context, item interface{}) error {
		attempts = attempts - 1 // attempts=5->4, 4->3, 3->2, 2->1, 1->0, 0->-1 etc.
		if attempts > 0 {       // attempts=4,3,2,1 will fail
			return fmt.Errorf("only a test")
		}
		// attempts=0 will succeed
		close(done)
		return nil
	},
		OptRetryQueueMaxAttempts(5),
		OptRetryQueueRetryWait(0),
	)

	go rtq.Start()
	<-rtq.NotifyStarted()
	defer rtq.Stop()

	// add a payload
	rtq.Add(context.Background(), "test payload")

	<-done
	assert.Zero(attempts)
}
