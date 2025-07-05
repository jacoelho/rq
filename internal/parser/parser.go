// Package parser provides YAML parsing functionality for HTTP workflow steps.

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
	Method   string            `yaml:"method"`             // HTTP method (GET, POST, etc.)
	URL      string            `yaml:"url"`                // Request URL
	Headers  map[string]string `yaml:"headers,omitempty"`  // HTTP headers
	Query    map[string]string `yaml:"query,omitempty"`    // Query parameters
	Options  Options           `yaml:"options,omitempty"`  // Request options
	Body     string            `yaml:"body,omitempty"`     // Request body
	Asserts  Asserts           `yaml:"asserts,omitempty"`  // Response assertions
	Captures *Captures         `yaml:"captures,omitempty"` // Data capture rules
}

// Options configures retry and redirect behavior for a step.
type Options struct {
	Retries        int   `yaml:"retries,omitempty"`         // Number of retry attempts
	FollowRedirect *bool `yaml:"follow_redirect,omitempty"` // Whether to follow redirects
}

// StatusAssert represents an assertion on the HTTP status code.
type StatusAssert struct {
	Predicate `yaml:",inline"`
}

// HeaderAssert represents an assertion on a specific HTTP header.
// It combines a header name with a predicate for flexible header validation.
type HeaderAssert struct {
	Name      string    `yaml:"name"`    // Header name to check
	Predicate Predicate `yaml:",inline"` // Predicate for header value validation
}

// CertificateAssert represents an assertion on SSL certificate information.
// It allows validation of certificate fields like Subject, Issuer, ExpireDate, and SerialNumber.
type CertificateAssert struct {
	Name      string    `yaml:"name"`    // Certificate field to check (subject, issuer, expire_date, serial_number)
	Predicate Predicate `yaml:",inline"` // Predicate for certificate field validation
}

// JSONPathAssert represents an assertion on a JSONPath expression.
// It allows validation of specific data extracted from response content.
type JSONPathAssert struct {
	Path      string    `yaml:"path"`    // JSONPath expression
	Predicate Predicate `yaml:",inline"` // Predicate for extracted value validation
}

// XPathAssert represents an assertion on an XPath expression.
// It allows validation of specific data extracted from XML response content.
type XPathAssert struct {
	Path      string    `yaml:"path"`    // XPath expression
	Predicate Predicate `yaml:",inline"` // Predicate for extracted value validation
}

// StatusCapture represents a capture of the HTTP status code.
type StatusCapture struct {
	Name   string `yaml:"name"`   // Variable name to store the captured status
	Redact bool   `yaml:"redact"` // Whether to add this value to secrets map for redaction
}

// HeaderCapture represents a capture of a specific HTTP header.
type HeaderCapture struct {
	Name       string `yaml:"name"`        // Variable name to store the captured value
	HeaderName string `yaml:"header_name"` // Header name to capture
	Redact     bool   `yaml:"redact"`      // Whether to add this value to secrets map for redaction
}

// CertificateCapture represents a capture of SSL certificate information.
type CertificateCapture struct {
	Name             string `yaml:"name"`              // Variable name to store the captured value
	CertificateField string `yaml:"certificate_field"` // Certificate field to capture
	Redact           bool   `yaml:"redact"`            // Whether to add this value to secrets map for redaction
}

// JSONPathCapture represents a capture using JSONPath expressions.
type JSONPathCapture struct {
	Name   string `yaml:"name"`   // Variable name to store the captured value
	Path   string `yaml:"path"`   // JSONPath expression
	Redact bool   `yaml:"redact"` // Whether to add this value to secrets map for redaction
}

// RegexCapture represents a capture using regular expressions.
type RegexCapture struct {
	Name    string `yaml:"name"`    // Variable name to store the captured value
	Pattern string `yaml:"pattern"` // Regular expression pattern
	Group   int    `yaml:"group"`   // Capture group number (0 for full match)
	Redact  bool   `yaml:"redact"`  // Whether to add this value to secrets map for redaction
}

// BodyCapture represents a capture of the entire response body.
type BodyCapture struct {
	Name   string `yaml:"name"`   // Variable name to store the captured body
	Redact bool   `yaml:"redact"` // Whether to add this value to secrets map for redaction
}

// Asserts groups all supported assertion types for a step.
// Each assertion type validates different aspects of the HTTP response.
type Asserts struct {
	Status      []StatusAssert      `yaml:"status,omitempty"`      // Status code assertions
	Headers     []HeaderAssert      `yaml:"headers,omitempty"`     // Header assertions
	Certificate []CertificateAssert `yaml:"certificate,omitempty"` // SSL certificate assertions
	JSONPath    []JSONPathAssert    `yaml:"jsonpath,omitempty"`    // JSONPath assertions
	XPath       []XPathAssert       `yaml:"xpath,omitempty"`       // XPath assertions
}

// Captures groups all supported capture types for a step.
// Each capture type extracts different aspects of the HTTP response.
type Captures struct {
	Status      []StatusCapture      `yaml:"status,omitempty"`      // Status code captures
	Headers     []HeaderCapture      `yaml:"headers,omitempty"`     // Header captures
	Certificate []CertificateCapture `yaml:"certificate,omitempty"` // SSL certificate captures
	JSONPath    []JSONPathCapture    `yaml:"jsonpath,omitempty"`    // JSONPath captures
	Regex       []RegexCapture       `yaml:"regex,omitempty"`       // Regex captures
	Body        []BodyCapture        `yaml:"body,omitempty"`        // Body captures
}

// UnmarshalYAML implements custom YAML unmarshaling for HeaderAssert.
// It separates the header name from the predicate fields during parsing.
func (h *HeaderAssert) UnmarshalYAML(node ast.Node) error {
	return unmarshalAssertWithField(node, "name", &h.Name, &h.Predicate, "HeaderAssert")
}

// UnmarshalYAML implements custom YAML unmarshaling for CertificateAssert.
// It separates the certificate field from the predicate fields during parsing.
func (c *CertificateAssert) UnmarshalYAML(node ast.Node) error {
	return unmarshalAssertWithField(node, "name", &c.Name, &c.Predicate, "CertificateAssert")
}

// UnmarshalYAML implements custom YAML unmarshaling for JSONPathAssert.
// It separates the path from the predicate fields during parsing.
func (p *JSONPathAssert) UnmarshalYAML(node ast.Node) error {
	return unmarshalAssertWithField(node, "path", &p.Path, &p.Predicate, "JSONPathAssert")
}

// UnmarshalYAML implements custom YAML unmarshaling for XPathAssert.
// It separates the path from the predicate fields during parsing.
func (x *XPathAssert) UnmarshalYAML(node ast.Node) error {
	return unmarshalAssertWithField(node, "path", &x.Path, &x.Predicate, "XPathAssert")
}

// unmarshalAssertWithField is a helper function to reduce code duplication
// in assertion unmarshaling. It extracts a named field and unmarshals the predicate.
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
