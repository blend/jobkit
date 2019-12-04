package jobkit

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"strings"

	"github.com/blend/go-sdk/validate"
)

type contextKeyParameters struct{}

// WithParameters adds job invocation parameters to a context.
func WithParameters(ctx context.Context, parameters ...Parameter) context.Context {
	return context.WithValue(ctx, contextKeyParameters{}, parameters)
}

// GetParameters gets parameters from a given context as a value.
func GetParameters(ctx context.Context) []Parameter {
	if value := ctx.Value(contextKeyParameters{}); value != nil {
		if typed, ok := value.([]Parameter); ok {
			return typed
		}
	}
	return nil
}

// Parameters is a collection of parameters.
type Parameters []Parameter

// Validate returns the result of calling validate on all params.
func (p Parameters) Validate() error {
	var err error
	for _, param := range p {
		if err = param.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Parameter is an option for a job invocation.
type Parameter struct {
	Label    string                  `yaml:"label"` // label will be a descriptive label next to the input
	Name     string                  `yaml:"name"`  // name is the post form key on submission
	Text     *ParameterTextInput     `yaml:"text"`
	Select   *ParameterSelectInput   `yaml:"select"`
	Checkbox *ParameterCheckboxInput `yaml:"checkbox"`
}

// Validate validates the parameter config.
func (p Parameter) Validate() error {
	return validate.First(
		validate.String(&p.Name).Required(),
		validate.Some(p.Text, p.Checkbox, p.Select).OneNotNil(),
		validate.When(func() bool { return p.Text != nil }, p.Text.Validate),
		validate.When(func() bool { return p.Select != nil }, p.Select.Validate),
		validate.When(func() bool { return p.Checkbox != nil }, p.Checkbox.Validate),
	)
}

// RenderLabel returns the html string for the input label.
func (p Parameter) RenderLabel() template.HTML {
	if p.Label != "" {
		return template.HTML(fmt.Sprintf("<label>%s</label>", p.Label))
	}
	return ""
}

// RenderInput returns the html string for the input.
func (p Parameter) RenderInput() template.HTML {
	var attributes []string
	if p.Name != "" {
		attributes = append(attributes, htmlAttr("name", p.Name))
	}
	if p.Text != nil {
		return p.Text.RenderInput(attributes...)
	} else if p.Checkbox != nil {
		return p.Checkbox.RenderInput(attributes...)
	} else if p.Select != nil {
		return p.Select.RenderInput(attributes...)
	}
	return ""
}

// ParameterTextInput is the input form of a parameter of type `text`.
type ParameterTextInput struct {
	Placeholder string
	Value       string
	Default     string
	Required    bool
	Password    bool
}

// RenderInput returns the html string for the input.
func (pti ParameterTextInput) RenderInput(attributes ...string) template.HTML {
	if pti.Password {
		attributes = append(attributes, htmlAttr("type", "password"))
	} else {
		attributes = append(attributes, htmlAttr("type", "text"))
	}
	// input placeholder
	if pti.Placeholder != "" {
		attributes = append(attributes, htmlAttr("placeholder", pti.Placeholder))
	}
	return template.HTML(fmt.Sprintf("<input %s />", strings.Join(attributes, " ")))
}

// Validate returns nil.
func (pti ParameterTextInput) Validate() error {
	return nil
}

// ParameterCheckboxInput is the input form of a parameter of type `checkbox`.
type ParameterCheckboxInput struct {
	Checked bool
}

// Validate returns nil.
func (pci ParameterCheckboxInput) Validate() error {
	return nil
}

// RenderInput returns the html string for the input.
func (pci ParameterCheckboxInput) RenderInput(attributes ...string) template.HTML {
	attributes = append(attributes, htmlAttr("type", "checkbox"))
	if pci.Checked {
		attributes = append(attributes, htmlAttr("checked", "true"))
	}
	return template.HTML(fmt.Sprintf("<input %s/>", strings.Join(attributes, " ")))
}

// ParameterSelectInput is the input form of a parameter of type `select`.
type ParameterSelectInput []ParamterSelectInputOption

// Validate returns nil.
func (psi ParameterSelectInput) Validate() error {
	return nil
}

// RenderInput returns the html string for the input.
func (psi ParameterSelectInput) RenderInput(attributes ...string) template.HTML {
	var options []string
	for _, option := range psi {
		options = append(options, fmt.Sprintf("<option %s>%s</option>", htmlAttr("value", option.Value), html.EscapeString(option.Text)))
	}
	return template.HTML(
		fmt.Sprintf(
			"<select %s>%s</select>",
			strings.Join(attributes, " "),
			strings.Join(options, ""),
		),
	)
}

// ParamterSelectInputOption is an option for a select input.
type ParamterSelectInputOption struct {
	Value string
	Text  string
}

func htmlAttr(name, value string) string {
	return name + "=\"" + html.EscapeString(value) + "\""
}
