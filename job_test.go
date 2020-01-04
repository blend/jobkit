package jobkit

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/blend/go-sdk/email"

	"github.com/blend/go-sdk/assert"
	"github.com/blend/go-sdk/cron"
	"github.com/blend/go-sdk/ref"
	"github.com/blend/go-sdk/slack"
	"github.com/blend/go-sdk/uuid"
)

func scheduleProvider(schedule cron.Schedule) func() cron.Schedule {
	return func() cron.Schedule {
		return schedule
	}
}

func TestNewJob(t *testing.T) {
	assert := assert.New(t)

	cfg := JobConfig{
		JobConfig: cron.JobConfig{
			Name:    "test",
			Timeout: 2 * time.Second,
		},
		Schedule: "@every 1s",
	}
	var didCallAction bool
	action := func(_ context.Context) error {
		didCallAction = true
		return nil
	}

	job, err := NewJob(cfg, action, OptParsedSchedule("@every 2s"))
	assert.Nil(err)
	assert.NotNil(job)
	assert.Equal(2*time.Second, job.CompiledSchedule.(cron.IntervalSchedule).Every)
	assert.Nil(job.Action(context.Background()))
	assert.True(didCallAction)
}

func TestWrapJob(t *testing.T) {
	assert := assert.New(t)

	job := cron.NewJob(
		cron.OptJobName("test"),
		cron.OptJobAction(func(_ context.Context) error {
			return nil
		}),
	)
	wrapped := WrapJob(job)
	assert.Equal(wrapped.Name(), job.Name())
	assert.NotNil(job.Action)
}

func TestJobLifecycleHooksNotificationsSlack(t *testing.T) {
	assert := assert.New(t)

	ctx := cron.WithJobInvocation(context.Background(), &cron.JobInvocation{
		ID:      uuid.V4().String(),
		JobName: "test-job",
	})

	slackMessages := make(chan slack.Message, 16)

	job := &Job{
		Config:      JobConfig{},
		SlackClient: slack.MockWebhookSender(slackMessages),
	}

	job.OnBegin(ctx)
	assert.Empty(slackMessages)

	job.OnComplete(ctx)
	assert.Empty(slackMessages)

	job.OnFailure(ctx)
	assert.NotEmpty(slackMessages)

	job.OnCancellation(ctx)
	assert.NotEmpty(slackMessages)

	job.OnBroken(ctx)
	assert.NotEmpty(slackMessages)

	job.OnFixed(ctx)
	assert.NotEmpty(slackMessages)
}

func TestJobLifecycleHooksNotificationsSetDisabled(t *testing.T) {
	assert := assert.New(t)

	ctx := cron.WithJobInvocation(context.Background(), &cron.JobInvocation{
		ID:      uuid.V4().String(),
		JobName: "test-job",
	})

	slackMessages := make(chan slack.Message, 1)

	job := &Job{
		SlackClient: slack.MockWebhookSender(slackMessages),
		Config: JobConfig{
			Notifications: JobNotificationsConfig{
				OnBegin:        ref.Bool(false),
				OnComplete:     ref.Bool(false),
				OnFailure:      ref.Bool(false),
				OnBroken:       ref.Bool(false),
				OnFixed:        ref.Bool(false),
				OnCancellation: ref.Bool(false),
			},
		},
	}

	job.OnBegin(ctx)
	assert.Empty(slackMessages)

	job.OnComplete(ctx)
	assert.Empty(slackMessages)

	job.OnFailure(ctx)
	assert.Empty(slackMessages)

	job.OnCancellation(ctx)
	assert.Empty(slackMessages)

	job.OnBroken(ctx)
	assert.Empty(slackMessages)

	job.OnFixed(ctx)
	assert.Empty(slackMessages)
}

func TestJobLifecycleHooksNotificationsSetEnabled(t *testing.T) {
	assert := assert.New(t)

	ctx := cron.WithJobInvocation(context.Background(), &cron.JobInvocation{
		ID:      uuid.V4().String(),
		JobName: "test-job",
		Err:     fmt.Errorf("only a test"),
	})

	slackMessages := make(chan slack.Message, 6)

	job := &Job{
		SlackClient: slack.MockWebhookSender(slackMessages),
		Config: JobConfig{
			Notifications: JobNotificationsConfig{
				OnBegin:        ref.Bool(true),
				OnComplete:     ref.Bool(true),
				OnFailure:      ref.Bool(true),
				OnBroken:       ref.Bool(true),
				OnFixed:        ref.Bool(true),
				OnCancellation: ref.Bool(true),
			},
		},
	}
	job.OnBegin(ctx)
	job.OnComplete(ctx)
	job.OnFailure(ctx)
	job.OnCancellation(ctx)
	job.OnBroken(ctx)
	job.OnFixed(ctx)

	assert.Len(slackMessages, 6)

	msg := <-slackMessages
	assert.Contains(msg.Attachments[0].Text, "cron.begin")

	msg = <-slackMessages
	assert.Contains(msg.Attachments[0].Text, "cron.complete")

	msg = <-slackMessages
	assert.Contains(msg.Attachments[0].Text, "cron.failed")

	msg = <-slackMessages
	assert.Contains(msg.Attachments[0].Text, "cron.cancelled")

	msg = <-slackMessages
	assert.Contains(msg.Attachments[0].Text, "cron.broken")

	msg = <-slackMessages
	assert.Contains(msg.Attachments[0].Text, "cron.fixed")
}

func TestJobLifecycleHooksEmailNotifications(t *testing.T) {
	assert := assert.New(t)

	ctx := cron.WithJobInvocation(context.Background(), &cron.JobInvocation{
		ID:      uuid.V4().String(),
		JobName: "test-job",
		Err:     fmt.Errorf("only a test"),
	})

	emailMessages := make(chan email.Message, 6)

	job := &Job{
		EmailClient: email.MockSender(emailMessages),
		Config: JobConfig{
			Notifications: JobNotificationsConfig{
				OnBegin:        ref.Bool(true),
				OnComplete:     ref.Bool(true),
				OnFailure:      ref.Bool(true),
				OnBroken:       ref.Bool(true),
				OnFixed:        ref.Bool(true),
				OnCancellation: ref.Bool(true),
			},
		},
	}

	job.OnBegin(ctx)
	job.OnComplete(ctx)
	job.OnFailure(ctx)
	job.OnCancellation(ctx)
	job.OnBroken(ctx)
	job.OnFixed(ctx)

	assert.Len(emailMessages, 6)

	msg := <-emailMessages
	assert.Contains(msg.Subject, "cron.begin")

	msg = <-emailMessages
	assert.Contains(msg.Subject, "cron.complete")

	msg = <-emailMessages
	assert.Contains(msg.Subject, "cron.failed")

	msg = <-emailMessages
	assert.Contains(msg.Subject, "cron.cancelled")

	msg = <-emailMessages
	assert.Contains(msg.Subject, "cron.broken")

	msg = <-emailMessages
	assert.Contains(msg.Subject, "cron.fixed")
}

func TestJobLifecycleHooksWebhookNotifications(t *testing.T) {
	assert := assert.New(t)

	ctx := cron.WithJobInvocation(context.Background(), &cron.JobInvocation{
		ID:      uuid.V4().String(),
		JobName: "test-job",
		Err:     fmt.Errorf("only a test"),
	})

	webhooks := make(chan *http.Request, 6)

	hookServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		webhooks <- req
		fmt.Fprintf(rw, "OK!\n")
	}))

	job := &Job{
		WebhookDefaults: Webhook{
			URL: hookServer.URL,
		},
		Config: JobConfig{
			Notifications: JobNotificationsConfig{
				OnBegin:        ref.Bool(true),
				OnComplete:     ref.Bool(true),
				OnFailure:      ref.Bool(true),
				OnBroken:       ref.Bool(true),
				OnFixed:        ref.Bool(true),
				OnCancellation: ref.Bool(true),
			},
		},
	}

	job.OnBegin(ctx)
	job.OnComplete(ctx)
	job.OnFailure(ctx)
	job.OnCancellation(ctx)
	job.OnBroken(ctx)
	job.OnFixed(ctx)

	assert.Len(webhooks, 6)
}

func TestJobHistoryProvider(t *testing.T) {
	assert := assert.New(t)

	tmpdir, err := ioutil.TempDir("", "gosdk_jobkit")
	assert.Nil(err)
	defer os.RemoveAll(tmpdir)

	cfg := JobConfig{
		JobConfig: cron.JobConfig{
			Name: "gosdk_jobkit",
		},
		HistoryPath: tmpdir,
	}
	job := &Job{
		Config:          cfg,
		HistoryProvider: HistoryJSON{Config: cfg},
	}

	history := []cron.JobInvocation{
		createTestCompleteJobInvocation("test0", 100*time.Millisecond),
		createTestCompleteJobInvocation("test0", 200*time.Millisecond),
		createTestFailedJobInvocation("test0", 100*time.Millisecond, fmt.Errorf("this is only a test")),
	}

	err = job.PersistHistory(context.Background(), history)
	assert.Nil(err)

	returned, err := job.RestoreHistory(context.Background())
	assert.Nil(err)
	assert.Len(returned, 3)
}
