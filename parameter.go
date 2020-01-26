package jobkit

import (
	"fmt"
	"html"
	"html/template"
	"strings"

	"github.com/blend/go-sdk/cron"
)

// DefaultParameterValues pulls the default parameter values
// off a set of jobkit parameters.
func DefaultParameterValues(params ...Parameter) cron.JobParameters {
	output := make(cron.JobParameters)
	for _, param := range params {
		output[param.Name] = param.Value
	}
	return output
}

// Parameter is an option for a job invocation.
type Parameter struct {
	Label       string            `yaml:"label"`       // label will be a descriptive label next to the input
	Name        string            `yaml:"name"`        // name is the post form key on submission
	Required    bool              `yaml:"required"`    // indicates the value for this parameter must be set
	Placeholder string            `yaml:"placeholder"` // placeholder is used to show ghost text in an input.
	Value       string            `yaml:"value"`       // value is the default value or the provided value.
	Options     []ParameterOption `yaml:"options"`     // options are select input options; if they are set the param will be a select
}

// RenderLabel returns the html string for the input label.
func (p Parameter) RenderLabel(attributes ...string) template.HTML {
	if p.Label != "" {
		return template.HTML(fmt.Sprintf("<label %s>%s</label>", strings.Join(attributes, " "), p.Label))
	}
	return ""
}

// RenderInput returns the html string for the input.
func (p Parameter) RenderInput(attributes ...string) template.HTML {
	if p.Name != "" {
		attributes = append(attributes, htmlAttr("name", p.Name))
	}

	if len(p.Options) > 0 {
		var options []string
		for _, option := range p.Options {
			if p.Value != "" && p.Value == option.Value {
				options = append(options, fmt.Sprintf("<option %s selected>%s</option>", htmlAttr("value", option.Value), html.EscapeString(option.Text)))
			} else {
				options = append(options, fmt.Sprintf("<option %s>%s</option>", htmlAttr("value", option.Value), html.EscapeString(option.Text)))
			}
		}
		return template.HTML(
			fmt.Sprintf(
				"<select %s>%s</select>",
				strings.Join(attributes, " "),
				strings.Join(options, ""),
			),
		)
	}

	attributes = append(attributes, htmlAttr("type", "text"))
	if p.Value != "" {
		attributes = append(attributes, htmlAttr("value", p.Value))
	}
	if p.Placeholder != "" {
		attributes = append(attributes, htmlAttr("placeholder", p.Placeholder))
	}
	return template.HTML(fmt.Sprintf("<input %s />", strings.Join(attributes, " ")))
}

// ParameterOption is an option for a parameter.
type ParameterOption struct {
	Value string
	Text  string
}

func htmlAttr(name, value string) string {
	return name + "=\"" + html.EscapeString(value) + "\""
}
