package jobkit

import (
	"net/http"

	"github.com/blend/go-sdk/r2"
)

// Webhook is a notification type.
type Webhook struct {
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
}

// IsZero returns if the webhook is set or not.
func (wh Webhook) IsZero() bool {
	return wh.URL == ""
}

// MethodOrDefault returns the webhoook method.
func (wh Webhook) MethodOrDefault() string {
	if wh.Method != "" {
		return wh.Method
	}
	return r2.MethodGet
}

// Options realizes the webhook as a set of r2 options.
func (wh Webhook) Options(options ...r2.Option) []r2.Option {
	options = append([]r2.Option{
		r2.OptMethod(wh.MethodOrDefault()),
	}, options...)

	if len(wh.Headers) > 0 {
		for key, value := range wh.Headers {
			options = append(options, r2.OptHeaderValue(http.CanonicalHeaderKey(key), value))
		}
	}
	if wh.Body != "" {
		options = append(options, r2.OptBodyBytes([]byte(wh.Body)))
	}
	return options
}

// Request creates a new r2 request for the webhook.
func (wh Webhook) Request(options ...r2.Option) *r2.Request {
	return r2.New(wh.URL, wh.Options(options...)...)
}
