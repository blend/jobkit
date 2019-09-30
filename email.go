package jobkit

import (
	"github.com/blend/go-sdk/cron"
	"github.com/blend/go-sdk/email"
	"github.com/blend/go-sdk/template"
)

// NewEmailMessage returns a new email message.
func NewEmailMessage(flag string, emailDefaults email.Message, ji *cron.JobInvocation, options ...email.MessageOption) (email.Message, error) {
	message := email.Message{
		From: emailDefaults.From,
		To:   emailDefaults.To,
	}

	vars := map[string]interface{}{
		"flag":    flag,
		"jobName": ji.JobName,
		"state":   ji.State,
		"elapsed": ji.Elapsed,
		"err":     ji.Err,
	}
	if ji.Output != nil && len(ji.Output.Chunks) > 0 {
		vars["output"] = ji.Output.String()
	}

	var err error
	message.Subject, err = template.New().WithBody(DefaultEmailSubjectTemplate).WithVars(vars).ProcessString()
	if err != nil {
		return message, err
	}
	message.HTMLBody, err = template.New().WithBody(DefaultEmailHTMLBodyTemplate).WithVars(vars).ProcessString()
	if err != nil {
		return message, err
	}
	message.TextBody, err = template.New().WithBody(DefaultEmailTextBodyTemplate).WithVars(vars).ProcessString()
	if err != nil {
		return message, err
	}

	return email.ApplyMessageOptions(message, options...), nil
}

const (
	// DefaultEmailMimeType is the default email mime type.
	DefaultEmailMimeType = "text/plain"

	// DefaultEmailSubjectTemplate is the default subject template.
	DefaultEmailSubjectTemplate = `{{.Var "jobName" }} :: {{ .Var "flag" }}`

	// DefaultEmailHTMLBodyTemplate is the default email html body template.
	DefaultEmailHTMLBodyTemplate = `
<html xmlns="http://www.w3.org/1999/xhtml">
<head>
<title>{{ .Var "jobName" }} {{ .Var "state" "unknown" }}</title>
<meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
<meta http-equiv="X-UA-Compatible" content="IE=edge" />
<meta name="viewport" content="width=device-width, initial-scale=1.0 " />
<style>
.email-body {
	margin: 0;
	padding: 20px;
	font-family: sans-serif;
	font-size: 16pt;
}
</style>
</head>
<body class="email-body">
	<h3>{{ .Var "jobName" }} {{ .Var "state" "Unknown" }}</h3>
	<div class="email-details">
	{{ if .Var "err" }}
	<h4>Error</h4>
	<pre>{{ .Var "err" }}</pre>
	{{ end }}
	</div>
	{{ if .Var "output" }}
	<h4>Output</h4>
	<pre>{{ .Var "output" }}</pre>
	{{ end }}
</body>
</html>
`

	// DefaultEmailTextBodyTemplate is the default body template.
	DefaultEmailTextBodyTemplate = `{{ .Var "jobName" }} {{ .Var "state" }}
Elapsed: {{ .Var "elapsed" }}
{{ if .HasVar "err" }}Error: {{ .Var "err" }}{{end}}
{{ if .HasVar "output" }}Output:
{{ .Var "output" }}{{end}}
`
)
