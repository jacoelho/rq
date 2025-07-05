package parser

import (
	"fmt"
	"io"

	yaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

// ErrParser is the sentinel error for all parser-related failures.
// It allows error wrapping and consistent error checks using errors.Is().
var ErrParser = fmt.Errorf("parser error")

// Step represents a single HTTP workflow step, including request, assertions, and captures.
// Each step defines an HTTP operation with optional validation and data extraction.
type Step struct {
	Method   string            `yaml:"method"`
	URL      string            `yaml:"url"`
	Headers  map[string]string `yaml:"headers,omitempty"`
	Query    map[string]string `yaml:"query,omitempty"`
	Options  Options           `yaml:"options,omitempty"`
	Body     string            `yaml:"body,omitempty"`
	Asserts  Asserts           `yaml:"asserts,omitempty"`
	Captures *Captures         `yaml:"captures,omitempty"`
}

// Options configures retry and redirect behavior for a step.
type Options struct {
	Retries        int   `yaml:"retries,omitempty"`
	FollowRedirect *bool `yaml:"follow_redirect,omitempty"`
}

// StatusAssert represents an assertion on the HTTP status code.
type StatusAssert struct {
	Predicate `yaml:",inline"`
}

// HeaderAssert represents an assertion on a specific HTTP header.
// It combines a header name with a predicate for flexible header validation.
type HeaderAssert struct {
	Name      string    `yaml:"name"`
	Predicate Predicate `yaml:",inline"`
}

// CertificateAssert represents an assertion on SSL certificate information.
// It allows validation of certificate fields like Subject, Issuer, ExpireDate, and SerialNumber.
type CertificateAssert struct {
	Name      string    `yaml:"name"`
	Predicate Predicate `yaml:",inline"`
}

// JSONPathAssert represents an assertion on a JSONPath expression.
// It allows validation of specific data extracted from response content.
type JSONPathAssert struct {
	Path      string    `yaml:"path"`
	Predicate Predicate `yaml:",inline"`
}

// XPathAssert represents an assertion on an XPath expression.
// It allows validation of specific data extracted from XML response content.
type XPathAssert struct {
	Path      string    `yaml:"path"`
	Predicate Predicate `yaml:",inline"`
}

// StatusCapture represents a capture of the HTTP status code.
type StatusCapture struct {
	Name   string `yaml:"name"`
	Redact bool   `yaml:"redact"`
}

// HeaderCapture represents a capture of a specific HTTP header.
type HeaderCapture struct {
	Name       string `yaml:"name"`
	HeaderName string `yaml:"header_name"`
	Redact     bool   `yaml:"redact"`
}

// CertificateCapture represents a capture of SSL certificate information.
type CertificateCapture struct {
	Name             string `yaml:"name"`
	CertificateField string `yaml:"certificate_field"`
	Redact           bool   `yaml:"redact"`
}

// JSONPathCapture represents a capture using JSONPath expressions.
type JSONPathCapture struct {
	Name   string `yaml:"name"`
	Path   string `yaml:"path"`
	Redact bool   `yaml:"redact"`
}

// RegexCapture represents a capture using regular expressions.
type RegexCapture struct {
	Name    string `yaml:"name"`
	Pattern string `yaml:"pattern"`
	Group   int    `yaml:"group"`
	Redact  bool   `yaml:"redact"`
}

// BodyCapture represents a capture of the entire response body.
type BodyCapture struct {
	Name   string `yaml:"name"`
	Redact bool   `yaml:"redact"`
}

// Asserts groups all supported assertion types for a step.
// Each assertion type validates different aspects of the HTTP response.
type Asserts struct {
	Status      []StatusAssert      `yaml:"status,omitempty"`
	Headers     []HeaderAssert      `yaml:"headers,omitempty"`
	Certificate []CertificateAssert `yaml:"certificate,omitempty"`
	JSONPath    []JSONPathAssert    `yaml:"jsonpath,omitempty"`
	XPath       []XPathAssert       `yaml:"xpath,omitempty"`
}

// Captures groups all supported capture types for a step.
// Each capture type extracts different aspects of the HTTP response.
type Captures struct {
	Status      []StatusCapture      `yaml:"status,omitempty"`
	Headers     []HeaderCapture      `yaml:"headers,omitempty"`
	Certificate []CertificateCapture `yaml:"certificate,omitempty"`
	JSONPath    []JSONPathCapture    `yaml:"jsonpath,omitempty"`
	Regex       []RegexCapture       `yaml:"regex,omitempty"`
	Body        []BodyCapture        `yaml:"body,omitempty"`
}

// UnmarshalYAML implements custom YAML unmarshaling for HeaderAssert.
func (h *HeaderAssert) UnmarshalYAML(node ast.Node) error {
	return unmarshalAssertWithField(node, "name", &h.Name, &h.Predicate, "HeaderAssert")
}

// UnmarshalYAML implements custom YAML unmarshaling for CertificateAssert.
func (c *CertificateAssert) UnmarshalYAML(node ast.Node) error {
	return unmarshalAssertWithField(node, "name", &c.Name, &c.Predicate, "CertificateAssert")
}

// UnmarshalYAML implements custom YAML unmarshaling for JSONPathAssert.
func (p *JSONPathAssert) UnmarshalYAML(node ast.Node) error {
	return unmarshalAssertWithField(node, "path", &p.Path, &p.Predicate, "JSONPathAssert")
}

// UnmarshalYAML implements custom YAML unmarshaling for XPathAssert.
func (x *XPathAssert) UnmarshalYAML(node ast.Node) error {
	return unmarshalAssertWithField(node, "path", &x.Path, &x.Predicate, "XPathAssert")
}

// unmarshalAssertWithField is a helper function to reduce code duplication
func unmarshalAssertWithField(node ast.Node, fieldName string, fieldValue *string, predicate *Predicate, typeName string) error {
	mapNode, ok := node.(*ast.MappingNode)
	if !ok {
		return fmt.Errorf("%w: %s: expected mapping node", ErrParser, typeName)
	}

	var predNode ast.Node
	for _, valNode := range mapNode.Values {
		kNode, ok := valNode.Key.(*ast.StringNode)
		if !ok {
			return fmt.Errorf("%w: %s: key must be string", ErrParser, typeName)
		}

		if kNode.Value == fieldName {
			stringVal, ok := valNode.Value.(*ast.StringNode)
			if !ok {
				return fmt.Errorf("%w: %s: %s value must be string", ErrParser, typeName, fieldName)
			}
			*fieldValue = stringVal.Value
		} else {
			if predNode == nil {
				predNode = &ast.MappingNode{}
			}
			predMap := predNode.(*ast.MappingNode)
			predMap.Values = append(predMap.Values, valNode)
		}
	}

	if predNode != nil {
		if err := predicate.UnmarshalYAML(predNode); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrParser, typeName, err)
		}
	}

	if typeName == "HeaderAssert" && *fieldValue == "" {
		return fmt.Errorf("%w: %s: missing required '%s' field", ErrParser, typeName, fieldName)
	}

	if typeName == "CertificateAssert" && *fieldValue == "" {
		return fmt.Errorf("%w: %s: missing required '%s' field", ErrParser, typeName, fieldName)
	}

	return nil
}

// Parse decodes a YAML stream of steps.
func Parse(r io.Reader) ([]Step, error) {
	decoder := yaml.NewDecoder(r)
	var steps []Step

	if err := decoder.Decode(&steps); err != nil {
		return nil, fmt.Errorf("%w: failed to decode YAML: %v", ErrParser, err)
	}

	return steps, nil
}
