package sentry

import (
	"context"
	"net/http"
	"runtime"
	"time"

	raven "github.com/getsentry/sentry-go"

	"github.com/blend/go-sdk/ex"
	"github.com/blend/go-sdk/logger"
)

var (
	_ Sender = (*Client)(nil)
)

// MustNew returns a new client and panics on error.
func MustNew(cfg Config) *Client {
	c, err := New(cfg)
	if err != nil {
		panic(err)
	}
	return c
}

// New returns a new client.
func New(cfg Config) (*Client, error) {
	rc, err := raven.NewClient(
		raven.ClientOptions{
			Dsn:         cfg.DSN,
			Environment: cfg.EnvironmentOrDefault(),
			ServerName:  cfg.ServerNameOrDefault(),
			Dist:        cfg.DistOrDefault(),
			Release:     cfg.ReleaseOrDefault(),
		},
	)
	if err != nil {
		return nil, err
	}
	return &Client{
		Config: cfg,
		Client: rc,
	}, nil
}

// Client is a wrapper for the sentry-go client.
type Client struct {
	Config Config
	Client *raven.Client
}

// Notify sends a notification.
func (c Client) Notify(ctx context.Context, ee logger.ErrorEvent) {
	c.Client.CaptureEvent(errEvent(ctx, ee), nil, raven.NewScope())
	c.Client.Flush(time.Second)
}

func errEvent(ctx context.Context, ee logger.ErrorEvent) *raven.Event {
	return &raven.Event{
		Timestamp: logger.GetEventTimestamp(ctx, ee).Unix(),
		Level:     raven.Level(ee.GetFlag()),
		Tags:      errTags(ctx),
		Extra:     errExtra(ctx),
		Platform:  "go",
		Sdk: raven.SdkInfo{
			Name:    SDK,
			Version: raven.Version,
			Packages: []raven.SdkPackage{{
				Name:    SDK,
				Version: raven.Version,
			}},
		},
		Request: errRequest(ee),
		Message: ex.ErrClass(ee.Err).Error(),
		Exception: []raven.Exception{
			{
				Type:       ex.ErrClass(ee.Err).Error(),
				Value:      ex.ErrMessage(ee.Err),
				Stacktrace: errStackTrace(ee.Err),
			},
		},
	}
}

func errTags(ctx context.Context) map[string]string {
	return logger.GetLabels(ctx)
}

func errExtra(ctx context.Context) map[string]interface{} {
	return logger.GetAnnotations(ctx)
}

func errRequest(ee logger.ErrorEvent) (requestMeta raven.Request) {
	if ee.State == nil {
		return
	}
	typed, ok := ee.State.(*http.Request)
	if !ok {
		return
	}
	requestMeta = requestMeta.FromHTTPRequest(typed)
	return
}

func errStackTrace(err error) *raven.Stacktrace {
	if err != nil {
		return &raven.Stacktrace{Frames: errFrames(err)}
	}
	return nil
}

func errFrames(err error) []raven.Frame {
	stacktrace := ex.ErrStackTrace(err)
	if stacktrace == nil {
		return []raven.Frame{}
	}
	pointers, ok := stacktrace.(ex.StackPointers)
	if !ok {
		return []raven.Frame{}
	}

	var output []raven.Frame
	runtimeFrames := runtime.CallersFrames(pointers)

	for {
		callerFrame, more := runtimeFrames.Next()
		output = append([]raven.Frame{
			raven.NewFrame(callerFrame),
		}, output...)
		if !more {
			break
		}
	}

	return output
}
