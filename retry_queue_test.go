package jobkit

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/blend/go-sdk/assert"
)

func TestRetryQueue(t *testing.T) {
	assert := assert.New(t)

	attempts := 5
	wg := sync.WaitGroup{}
	wg.Add(attempts)
	rtq := NewRetryQueue(func(_ context.Context, item interface{}) error {
		defer wg.Done()
		if attempts > 0 {
			attempts = attempts - 1
			return fmt.Errorf("only a test")
		}
		return nil
	},
		OptRetryQueueMaxAttempts(5),
		OptRetryQueueRetryWait(0),
	)
	go rtq.Start()
	<-rtq.NotifyStarted()

	defer func() {
		rtq.Stop()
	}()

	rtq.Add(context.Background(), "test payload")
	wg.Wait()
	assert.Zero(attempts)
}
