package jobkit

import (
	"fmt"

	"github.com/blend/go-sdk/slack"
)

// NewSlackMessage returns a new job started message.
func NewSlackMessage(flag string, defaults slack.Message, ji *JobInvocation, options ...slack.MessageOption) slack.Message {
	message := defaults
	if ji.JobInvocation.Err != nil {
		message.Attachments = append(message.Attachments,
			slack.MessageAttachment{
				Text:  fmt.Sprintf("%s %s", ji.JobInvocation.JobName, flag),
				Color: "#ff0000",
			},
			slack.MessageAttachment{
				Text:  fmt.Sprintf("error: %+v", ji.JobInvocation.Err),
				Color: "#ff0000",
			},
		)
	} else {
		message.Attachments = append(message.Attachments,
			slack.MessageAttachment{
				Text:  fmt.Sprintf("%s %s", ji.JobInvocation.JobName, flag),
				Color: "#00ff00",
			},
		)
	}

	if ji.JobInvocation.Elapsed() > 0 {
		message.Attachments = append(message.Attachments,
			slack.MessageAttachment{
				Text: fmt.Sprintf("%v elapsed", ji.JobInvocation.Elapsed()),
			},
		)
	}

	return slack.ApplyMessageOptions(message, options...)
}
