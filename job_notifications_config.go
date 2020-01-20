package jobkit

import (
	"time"

	"github.com/blend/go-sdk/email"
	"github.com/blend/go-sdk/slack"
)

// JobNotificationsConfig are the notification options for a job.
type JobNotificationsConfig struct {
	Slack slack.Message `yaml:"slack"`
	// Email holds the message defaults for email notifications.
	Email email.Message `yaml:"email"`
	// Webhook set a webhook target for notifications.
	Webhook Webhook `yaml:"webhook"`

	// MaxRetries is the maximum number of retries before we give up on a notification.
	MaxRetries int `yaml:"maxRetries"`
	// RetryWait is the time between attempts.
	RetryWait time.Duration `yaml:"retryWait"`

	// OnBegin governs if we should send notifications job start.
	OnBegin *bool `yaml:"onBegin"`
	// OnComplete governs if we should send notifications on any success.
	OnComplete *bool `yaml:"onComplete"`
	// OnSuccess governs if we should send notifications on any success.
	OnSuccess *bool `yaml:"onSuccess"`
	// OnFailure governs if we should send notifications on any failure.
	OnError *bool `yaml:"onError"`
	// OnCancellation governs if we should send notifications on cancellation.
	OnCancellation *bool `yaml:"onCancellation"`
	// OnBroken governs if we should send notifications on a success => failure transition.
	OnBroken *bool `yaml:"onBroken"`
	// OnFixed governs if we should send notifications on a failure => success transition.
	OnFixed *bool `yaml:"onFixed"`
	// OnEnabled governs if we should send notifications when a job is enabled.
	OnEnabled *bool `yaml:"onEnabled"`
	// OnDisabled governs if we should send notifications when a job is disabled.
	OnDisabled *bool `yaml:"onDisabled"`
}

// OnBeginOrDefault returns a value or a default.
func (jnc JobNotificationsConfig) OnBeginOrDefault() bool {
	if jnc.OnBegin != nil {
		return *jnc.OnBegin
	}
	return false
}

// OnCompleteOrDefault returns a value or a default.
func (jnc JobNotificationsConfig) OnCompleteOrDefault() bool {
	if jnc.OnComplete != nil {
		return *jnc.OnComplete
	}
	return false
}

// OnSuccessOrDefault returns a value or a default.
func (jnc JobNotificationsConfig) OnSuccessOrDefault() bool {
	if jnc.OnSuccess != nil {
		return *jnc.OnSuccess
	}
	return false
}

// OnErrorOrDefault returns a value or a default.
func (jnc JobNotificationsConfig) OnErrorOrDefault() bool {
	if jnc.OnError != nil {
		return *jnc.OnError
	}
	return true
}

// OnCancellationOrDefault returns a value or a default.
func (jnc JobNotificationsConfig) OnCancellationOrDefault() bool {
	if jnc.OnCancellation != nil {
		return *jnc.OnCancellation
	}
	return true
}

// OnBrokenOrDefault returns a value or a default.
func (jnc JobNotificationsConfig) OnBrokenOrDefault() bool {
	if jnc.OnBroken != nil {
		return *jnc.OnBroken
	}
	return true
}

// OnFixedOrDefault returns a value or a default.
func (jnc JobNotificationsConfig) OnFixedOrDefault() bool {
	if jnc.OnFixed != nil {
		return *jnc.OnFixed
	}
	return true
}

// OnEnabledOrDefault returns a value or a default.
func (jnc JobNotificationsConfig) OnEnabledOrDefault() bool {
	if jnc.OnEnabled != nil {
		return *jnc.OnEnabled
	}
	return false
}

// OnDisabledOrDefault returns a value or a default.
func (jnc JobNotificationsConfig) OnDisabledOrDefault() bool {
	if jnc.OnDisabled != nil {
		return *jnc.OnDisabled
	}
	return false
}
