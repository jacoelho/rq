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

#### Flags

- `--debug`                 Enable debug output showing request and response details
- `--secret NAME=VALUE`     Secret in format name=value (can be used multiple times)
- `--secret-file FILE`      Path to key=value file containing secrets
- `--secret-salt SALT`      Salt to use for secret redaction hashes (default: current date)
- `--rate-limit N`          Rate limit in requests per second (0 for unlimited)
- `--repeat N`              Number of additional times to repeat after first run (negative for infinite)
- `--insecure`              Skip TLS certificate verification
- `--cacert FILE`           Path to CA certificate file for TLS verification
- `--timeout DURATION`      HTTP request timeout (default: 30s)
- `-h, --help`              Show help message
- `-v, --version`           Show version information

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

## Query Parameters

Add query parameters to requests:

```yaml
- method: GET
  url: https://httpbin.org/get
  query:
    search: Install Linux
    order: newest
    limit: 10
  asserts:
    jsonpath:
      - path: $.args.search
        op: equals
        value: "Install Linux"
```

Query parameters support template variables:

```yaml
- method: GET
  url: https://httpbin.org/get
  query:
    user_id: "{{uuidv4}}"
    timestamp: "{{timestamp}}"
    search: "{{.search_term}}"
```

Query parameters are appended to existing URL parameters:

```yaml
- method: GET
  url: https://httpbin.org/get?existing=value&fixed=param
  query:
    search: Install Linux
    order: newest
  # Results in: https://httpbin.org/get?existing=value&fixed=param&search=Install+Linux&order=newest
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

The `examples/` directory contains comprehensive examples.

Run all examples:

```bash
make examples
```

## Debug Mode

Enable debug output to see detailed request and response information:

```bash
rq --debug test.yaml
```

Debug mode shows:
- Complete HTTP request headers and body (with secrets redacted)
- Complete HTTP response headers and body (with secrets redacted)
- Template variable values
- Assertion evaluation details

### Secret Redaction in Debug Output

When debug mode is enabled, any secret values provided via `--secret` or `--secret-file` are automatically redacted in all debug output (requests and responses). Instead of showing the actual secret, rq replaces it with a deterministic hash in the format:

```
[S256:xxxxxxxxxxxxxxxx]
```

Where `xxxxxxxxxxxxxxxx` is the first 16 hex digits of the SHA256 hash of the salt (see `--secret-salt`) concatenated with the secret value. This ensures secrets are never leaked in logs, while still allowing you to distinguish different secrets.

- The salt defaults to the current date, but can be set explicitly with `--secret-salt SALT` for reproducible output.
- Example: If you pass `--secret api_key=supersecret` and `--secret-salt my-salt`, any occurrence of `supersecret` in debug output will be replaced with `[S256:xxxxxxxxxxxxxxxx]` (the hash of `my-salt` + `supersecret`).

**Note:** The actual HTTP requests sent to the server use the real secret values. Only the debug output is redacted.

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
