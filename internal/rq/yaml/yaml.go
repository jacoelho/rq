package yaml

import (
	"fmt"
	"io"

	"github.com/goccy/go-yaml"
	"github.com/jacoelho/rq/internal/rq/model"
)

// Parse decodes rq YAML test files into runtime steps.
func Parse(r io.Reader) ([]model.Step, error) {
	return model.Parse(r)
}

// EncodeStep renders a single step as rq YAML file content.
func EncodeStep(step model.Step) ([]byte, error) {
	payload, err := yaml.Marshal([]stepYAML{mapStep(step)})
	if err != nil {
		return nil, fmt.Errorf("encode YAML: %w", err)
	}

	return payload, nil
}

type stepYAML struct {
	Method   string          `yaml:"method"`
	URL      string          `yaml:"url"`
	When     string          `yaml:"when,omitempty"`
	Headers  model.KeyValues `yaml:"headers,omitempty"`
	Query    model.KeyValues `yaml:"query,omitempty"`
	Options  model.Options   `yaml:"options,omitempty"`
	Body     string          `yaml:"body,omitempty"`
	BodyFile string          `yaml:"body_file,omitempty"`
	Asserts  assertsYAML     `yaml:"asserts,omitempty"`
	Captures *model.Captures `yaml:"captures,omitempty"`
}

type assertsYAML struct {
	Status      []statusAssertYAML      `yaml:"status,omitempty"`
	Headers     []headerAssertYAML      `yaml:"headers,omitempty"`
	Certificate []certificateAssertYAML `yaml:"certificate,omitempty"`
	JSONPath    []jsonPathAssertYAML    `yaml:"jsonpath,omitempty"`
}

type statusAssertYAML struct {
	Op    string     `yaml:"op"`
	Value *yamlValue `yaml:"value,omitempty"`
}

type headerAssertYAML struct {
	Name  string     `yaml:"name"`
	Op    string     `yaml:"op"`
	Value *yamlValue `yaml:"value,omitempty"`
}

type certificateAssertYAML struct {
	Name  string     `yaml:"name"`
	Op    string     `yaml:"op"`
	Value *yamlValue `yaml:"value,omitempty"`
}

type jsonPathAssertYAML struct {
	Path  string     `yaml:"path"`
	Op    string     `yaml:"op"`
	Value *yamlValue `yaml:"value,omitempty"`
}

type yamlValue struct {
	Value any
}

func (v *yamlValue) MarshalYAML() (any, error) {
	return v.Value, nil
}

func mapStep(step model.Step) stepYAML {
	mapped := stepYAML{
		Method:   step.Method,
		URL:      step.URL,
		When:     step.When,
		Headers:  step.Headers,
		Query:    step.Query,
		Options:  step.Options,
		Body:     step.Body,
		BodyFile: step.BodyFile,
		Asserts:  mapAsserts(step.Asserts),
		Captures: step.Captures,
	}

	return mapped
}

func mapAsserts(asserts model.Asserts) assertsYAML {
	out := assertsYAML{
		Status:      make([]statusAssertYAML, 0, len(asserts.Status)),
		Headers:     make([]headerAssertYAML, 0, len(asserts.Headers)),
		Certificate: make([]certificateAssertYAML, 0, len(asserts.Certificate)),
		JSONPath:    make([]jsonPathAssertYAML, 0, len(asserts.JSONPath)),
	}

	for _, assert := range asserts.Status {
		out.Status = append(out.Status, statusAssertYAML{
			Op:    assert.Predicate.Operation,
			Value: predicateValue(assert.Predicate),
		})
	}

	for _, assert := range asserts.Headers {
		out.Headers = append(out.Headers, headerAssertYAML{
			Name:  assert.Name,
			Op:    assert.Predicate.Operation,
			Value: predicateValue(assert.Predicate),
		})
	}

	for _, assert := range asserts.Certificate {
		out.Certificate = append(out.Certificate, certificateAssertYAML{
			Name:  assert.Name,
			Op:    assert.Predicate.Operation,
			Value: predicateValue(assert.Predicate),
		})
	}

	for _, assert := range asserts.JSONPath {
		out.JSONPath = append(out.JSONPath, jsonPathAssertYAML{
			Path:  assert.Path,
			Op:    assert.Predicate.Operation,
			Value: predicateValue(assert.Predicate),
		})
	}

	return out
}

func predicateValue(predicate model.Predicate) *yamlValue {
	if !predicate.HasValue {
		return nil
	}

	return &yamlValue{Value: predicate.Value}
}
