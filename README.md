# rq

A command-line HTTP testing tool for API workflows and automation.

*Inspired by [Hurl](https://hurl.dev/)*

## Overview

rq executes HTTP requests defined in YAML files with support for assertions, data capture, and multi-step workflows. It provides a simple way to test APIs, automate HTTP interactions, and validate responses.

## Installation

```bash
go install github.com/jacoelho/rq/cmd/rq@latest
```


## Quick Start

Create a simple test file:

```yaml
# test.yaml
- method: GET
  url: https://httpbin.org/json
  asserts:
    status:
      - op: equals
        value: 200
    jsonpath:
      - path: $.slideshow.title
        op: equals
        value: "Sample Slide Show"
```

Run the test:

```bash
rq test.yaml
```

## Basic Usage

### Command Line Options

```bash
rq [options] <file1.yaml> [file2.yaml...]
```

### YAML Structure

Each YAML file contains a list of HTTP steps:

```yaml
- method: GET|POST|PUT|PATCH|DELETE
  url: https://example.com/api
  headers:
    Content-Type: application/json
  body: |
    {"key": "value"}
  options:
    retries: 3
    follow_redirect: true
  asserts:
    # Response validation
  captures:
    # Data extraction
```

## HTTP Methods

All standard HTTP methods are supported:

```yaml
- method: GET
  url: https://httpbin.org/get

- method: POST
  url: https://httpbin.org/post
  headers:
    Content-Type: application/json
  body: |
    {"name": "John", "email": "john@example.com"}

- method: PUT
  url: https://httpbin.org/put
  body: "Updated content"

- method: DELETE
  url: https://httpbin.org/delete
```

## Assertions

### Status Code Assertions

```yaml
asserts:
  status:
    - op: equals
      value: 200
    - op: not_equals
      value: 404
    - op: regex
      value: "^2[0-9]{2}$"  # 2xx success codes
```

### Header Assertions

```yaml
asserts:
  headers:
    - name: Content-Type
      op: contains
      value: "application/json"
    - name: Server
      op: exists
```

### JSONPath Assertions

```yaml
asserts:
  jsonpath:
    - path: $.user.name
      op: equals
      value: "John Doe"
    - path: $.items
      op: length
      value: 5
    - path: $.status
      op: regex
      value: "^(active|inactive)$"
```

**Note**: For property names containing special characters (like hyphens), use bracket notation:
```yaml
asserts:
  jsonpath:
    - path: $.headers['Content-Type']
      op: contains
      value: "application/json"
    - path: $.headers['User-Agent']
      op: contains
      value: "Mozilla"
```

### Assertion Operators

- `equals` - Exact match
- `not_equals` - Not equal to
- `contains` - String contains substring
- `regex` - Regular expression match
- `exists` - Value is present
- `length` - Array/string length

## Data Capture

Extract data from responses for use in subsequent requests:

### Status Capture

```yaml
captures:
  status:
    - name: response_code
```

### Header Capture

```yaml
captures:
  headers:
    - name: auth_token
      header_name: Authorization
    - name: content_type
      header_name: Content-Type
```

### JSONPath Capture

```yaml
captures:
  jsonpath:
    - name: user_id
      path: $.user.id
    - name: session_token
      path: $.auth.token
```

**Note**: For property names containing special characters (like hyphens), use bracket notation:
```yaml
captures:
  jsonpath:
    - name: content_type
      path: $.headers['Content-Type']
    - name: user_agent
      path: $.headers['User-Agent']
```

### Regex Capture

```yaml
captures:
  regex:
    - name: version
      pattern: "version: (\\d+\\.\\d+\\.\\d+)"
      group: 1
```

### Certificate Capture

```yaml
captures:
  certificate:
    - name: cert_subject
      certificate_field: subject
    - name: cert_issuer
      certificate_field: issuer
    - name: cert_expiry
      certificate_field: expire_date
    - name: cert_serial
      certificate_field: serial_number
```

### Body Capture

```yaml
captures:
  body:
    - name: full_response
```

## Template Variables

Use captured data in subsequent requests:

```yaml
# First request captures data
- method: POST
  url: https://httpbin.org/post
  body: |
    {"username": "admin"}
  captures:
    jsonpath:
      - name: user_id
        path: $.json.username

# Second request uses captured data
- method: GET
  url: https://httpbin.org/headers
  headers:
    X-User-ID: "{{.user_id}}"
    Authorization: "Bearer {{.auth_token}}"
```

## Template Functions

Built-in functions for dynamic data generation:

```yaml
- method: POST
  url: https://httpbin.org/post
  body: |
    {
      "id": "{{uuidv4}}",
      "timestamp": "{{timestamp}}",
      "datetime": "{{now}}",
      "random": "{{randomString 8}}",
      "number": {{randomInt 1 100}},
      "encoded": "{{base64 "hello world"}}"
    }
```

Available functions:
- `uuidv4` / `uuid` - Generate UUID v4
- `now` - Current time in RFC3339 format
- `timestamp` - Current Unix timestamp
- `iso8601` - Current time in ISO 8601 format
- `rfc3339` - Current time in RFC3339 format
- `randomInt min max` - Random integer
- `randomString length` - Random string
- `base64 string` - Base64 encode
- `upper string` - Uppercase
- `lower string` - Lowercase
- `title string` - Title case
- `trim string` - Trim whitespace

## Request Options

### Retries

```yaml
- method: GET
  url: https://httpbin.org/status/500
  options:
    retries: 3
```

### Redirect Handling

```yaml
- method: GET
  url: https://httpbin.org/redirect/1
  options:
    follow_redirect: false
  asserts:
    status:
      - op: equals
        value: 302
```

## Form Data

Submit form data with different content types:

```yaml
- method: POST
  url: https://httpbin.org/post
  headers:
    Content-Type: application/x-www-form-urlencoded
  body: "name=John&email=john@example.com"
```

## Multi-Step Workflows

Chain multiple requests together:

```yaml
# Step 1: Login
- method: POST
  url: https://httpbin.org/post
  body: |
    {"username": "admin", "password": "secret"}
  captures:
    jsonpath:
      - name: session_token
        path: $.json.username

# Step 2: Get user data
- method: GET
  url: https://httpbin.org/headers
  headers:
    Authorization: "Bearer {{.session_token}}"
  asserts:
    status:
      - op: equals
        value: 200

# Step 3: Update user
- method: PUT
  url: https://httpbin.org/put
  headers:
    Authorization: "Bearer {{.session_token}}"
    Content-Type: application/json
  body: |
    {"name": "Updated Name"}
```

## Examples

The `examples/` directory contains comprehensive examples:

- `01-basic-get.yaml` - Simple GET request
- `02-post-with-json.yaml` - POST with JSON body
- `03-headers-and-auth.yaml` - Custom headers and authentication
- `04-captures-and-variables.yaml` - Data capture and variable usage
- `05-template-functions.yaml` - Built-in template functions
- `06-status-codes.yaml` - Testing different status codes
- `07-response-capture.yaml` - Response data capture
- `08-regex-capture.yaml` - Regular expression extraction
- `09-advanced-assertions.yaml` - Advanced assertion types
- `10-retries-and-options.yaml` - Retry behavior and options
- `11-form-data.yaml` - Form data submission
- `12-complex-workflow.yaml` - Multi-step workflow

Run all examples:

```bash
rq examples/*.yaml
```

## Debug Mode

Enable debug output to see detailed request and response information:

```bash
rq --debug test.yaml
```

Debug mode shows:
- Complete HTTP request headers and body
- Complete HTTP response headers and body
- Template variable values
- Assertion evaluation details

## Rate Limiting

Control request rate to avoid overwhelming servers:

```bash
rq --rate-limit 10 test.yaml  # 10 requests per second
```

## Repeated Execution

Run tests multiple times for load testing or reliability checks:

```bash
rq --repeat 100 test.yaml
```

## Exit Codes

- `0` - All tests passed
- `1` - Tests failed, configuration error, or parsing error

## Error Handling

rq provides detailed error messages for common issues:

- Invalid YAML syntax
- Network connection failures
- Assertion failures with expected vs actual values
- Template variable resolution errors
- JSONPath expression errors

## License

MIT License
