# rq

A command-line HTTP testing tool for API workflows and automation.

Inspired by [Hurl](https://hurl.dev/).

---

## What is rq?

rq runs HTTP requests defined in YAML files. It supports assertions, data capture, and multi-step workflows. Use it to test APIs, automate HTTP calls, and validate responses.

---

## Installation

```bash
go install github.com/jacoelho/rq/cmd/rq@latest
```

---

## Quick Start

Create a test file:

```yaml
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

Run it:

```bash
rq test.yaml
```

---

## Command Line Usage

```bash
rq [options] <file1.yaml> [file2.yaml...]
```

**Common options:**

| Flag                  | Description                                      |
|-----------------------|--------------------------------------------------|
| `--debug`             | Show request/response debug output               |
| `--secret NAME=VALUE` | Provide secret (can be used multiple times)      |
| `--secret-file FILE`  | Load secrets from file                           |
| `--secret-salt SALT`  | Salt for secret redaction hashes                 |
| `--rate-limit N`      | Requests per second (0 = unlimited)              |
| `--repeat N`          | Repeat test N times (negative = infinite)        |
| `--insecure`          | Skip TLS verification                            |
| `--cacert FILE`       | Custom CA certificate                            |
| `--timeout DURATION`  | Request timeout (default: 30s)                   |
| `-h, --help`          | Show help                                        |
| `-v, --version`       | Show version                                     |

---

## Writing Tests

Each YAML file contains a list of HTTP steps:

```yaml
- method: POST
  url: https://example.com/api
  headers:
    Content-Type: application/json
  body: |
    {"key": "value"}
  asserts:
    status:
      - op: equals
        value: 200
  captures:
    jsonpath:
      - name: token
        path: $.auth.token
        redact: true
```

---

## Features

### Supported HTTP Methods

```yaml
- method: GET|POST|PUT|PATCH|DELETE
  url: https://httpbin.org/...
```

### Query Parameters

```yaml
query:
  search: Install Linux
  order: newest
  limit: 10
```

You can use template variables:

```yaml
query:
  user_id: "{{uuidv4}}"
  timestamp: "{{timestamp}}"
```

---

### Assertions

Check status, headers, or JSONPath values:

```yaml
asserts:
  status:
    - op: equals
      value: 200
  headers:
    - name: Content-Type
      op: contains
      value: "application/json"
  jsonpath:
    - path: $.user.name
      op: equals
      value: "John Doe"
```

**Operators:** `equals`, `not_equals`, `contains`, `regex`, `exists`, `length`

---

### Data Capture

Extract data for use in later steps:

```yaml
captures:
  jsonpath:
    - name: session_token
      path: $.auth.token
      redact: true  # Redact in debug output
  headers:
    - name: content_type
      header_name: Content-Type
```

Other capture types: `status`, `regex`, `certificate`, `body`

---

### Using Captured Data

Use captured values in later requests:

```yaml
headers:
  Authorization: "Bearer {{.session_token}}"
```

---

### Template Functions

Generate dynamic values:

- `uuidv4` — Random UUID
- `now` — Current time (RFC3339)
- `timestamp` — Unix timestamp
- `randomInt min max` — Random integer
- `randomString length` — Random string
- `base64 string` — Base64 encode

Example:

```yaml
body: |
  {
    "id": "{{uuidv4}}",
    "created": "{{now}}"
  }
```

---

### Request Options

- **Retries:**  
  ```yaml
  options:
    retries: 3
  ```
- **Redirects:**  
  ```yaml
  options:
    follow_redirect: false
  ```

---

### Form Data

```yaml
headers:
  Content-Type: application/x-www-form-urlencoded
body: "name=John&email=john@example.com"
```

---

### Multi-Step Workflows

Chain requests and use captured data:

```yaml
- method: POST
  url: https://httpbin.org/post
  body: |
    {"username": "admin", "password": "secret"}
  captures:
    jsonpath:
      - name: session_token
        path: $.json.username

- method: GET
  url: https://httpbin.org/headers
  headers:
    Authorization: "Bearer {{.session_token}}"
```

---

## Debugging and Secret Redaction

- Run with `--debug` to see request/response details.
- Secrets and redacted captures are replaced with `[S256:xxxxxxxxxxxxxxxx]` in debug output.
- The real values are still used for requests and variable substitution.

---

## Examples

See the `examples/` directory for more sample files.

Run all examples:

```bash
make examples
```

---

## Other Features

- **Rate limiting:**  
  `rq --rate-limit 10 test.yaml`
- **Repeated execution:**  
  `rq --repeat 100 test.yaml`
- **Exit codes:**  
  `0` = success, `1` = failure or error

---

## Troubleshooting

rq provides clear error messages for:

- Invalid YAML
- Network failures
- Assertion failures
- Template errors
- JSONPath errors

---

## License

MIT License
