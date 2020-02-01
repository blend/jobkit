package slack

import (
	"context"
)

// NewMockWebhookSender creates a new mock sender.
func NewMockWebhookSender() MockWebhookSender {
	return MockWebhookSender(make(chan Message))
}

var (
	_ Sender = (*MockWebhookSender)(nil)
)

// MockWebhookSender is a mocked sender.
type MockWebhookSender chan Message

// Send sends a mocked message.
func (ms MockWebhookSender) Send(ctx context.Context, m Message) error {
	ms <- m
	return nil
}

// PostMessage sends a mocked message.
func (ms MockWebhookSender) PostMessage(channel, text string, options ...MessageOption) error {
	m := Message{
		Channel: channel,
		Text:    text,
	}
	for _, option := range options {
		option(&m)
	}

	ms <- m
	return nil
}

// PostMessageContext sends a mocked message.
func (ms MockWebhookSender) PostMessageContext(ctx context.Context, channel, text string, options ...MessageOption) error {
	m := Message{
		Channel: channel,
		Text:    text,
	}
	for _, option := range options {
		option(&m)
	}

	ms <- m
	return nil
}
