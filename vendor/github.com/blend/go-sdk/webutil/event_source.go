package webutil

import (
	"io"
	"net/http"

	"github.com/blend/go-sdk/ex"
	"github.com/blend/go-sdk/stringutil"
)

// NewEventSource returns a new event source.
func NewEventSource(output http.ResponseWriter) EventSource {
	return EventSource{Output: output}
}

// EventSource is a helper for writing event source info.
type EventSource struct {
	Output http.ResponseWriter
}

// StartSession starts an event source session.
func (es EventSource) StartSession() error {
	es.Output.Header().Set(HeaderContentType, "text/event-stream")
	es.Output.Header().Set(HeaderVary, "Content-Type")
	es.Output.WriteHeader(http.StatusOK)
	return es.Ping()
}

// Ping sends the ping heartbeat event.
func (es EventSource) Ping() error {
	return es.Event("ping")
}

// Event writes an event.
func (es EventSource) Event(name string) error {
	_, err := io.WriteString(es.Output, "event: "+name+"\n\n")
	if err != nil {
		return ex.New(err)
	}
	if typed, ok := es.Output.(http.Flusher); ok {
		typed.Flush()
	}
	return nil
}

// Data writes a data segment.
// It will slit lines on newline across multiple data events.
func (es EventSource) Data(data string) error {
	lines := stringutil.SplitLines(data, stringutil.OptSplitLinesIncludeEmptyLines(true))

	for _, line := range lines {
		_, err := io.WriteString(es.Output, "data: "+line+"\n")
		if err != nil {
			return ex.New(err)
		}
	}
	_, err := io.WriteString(es.Output, "\n")
	if err != nil {
		return ex.New(err)
	}
	if typed, ok := es.Output.(http.Flusher); ok {
		typed.Flush()
	}
	return nil
}

// EventData sends an event with a given set of data.
func (es EventSource) EventData(name, data string) error {
	_, err := io.WriteString(es.Output, "event: "+name+"\n")
	if err != nil {
		return ex.New(err)
	}
	return es.Data(data)
}
