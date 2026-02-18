package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jacoelho/rq/internal/exit"
	"github.com/jacoelho/rq/internal/results"
)

const (
	// DefaultTimeout is the default timeout for HTTP requests.
	DefaultTimeout = 30 * time.Second
)

// timeNow can be overridden in tests for deterministic behavior.
var timeNow = time.Now

var (
	ErrNoArguments           = errors.New("no arguments provided")
	ErrNoTestFiles           = errors.New("no test files specified")
	ErrInvalidSecretFormat   = errors.New("secret must be in format name=value")
	ErrEmptySecretName       = errors.New("secret name cannot be empty")
	ErrInvalidVariableFormat = errors.New("variable must be in format name=value")
	ErrEmptyVariableName     = errors.New("variable name cannot be empty")
	ErrInvalidOutputFormat   = errors.New("output format must be one of: text, json")
)

type Config struct {
	TestFiles []string
	Debug     bool
	Repeat    int // Additional iterations after first run (negative = infinite)

	Insecure       bool
	CACertFile     string
	RequestTimeout time.Duration
	RateLimit      float64 // Requests per second (0 = unlimited)
	OutputFormat   results.OutputFormat

	Secrets    map[string]any
	SecretFile string
	Variables  map[string]any
	SecretSalt string
}

func (c *Config) TLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: c.Insecure,
	}

	if c.CACertFile != "" {
		caCertPool, err := x509.SystemCertPool()
		if err != nil {
			caCertPool = x509.NewCertPool()
		}

		caCert, err := os.ReadFile(c.CACertFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate file %s: %w", c.CACertFile, err)
		}

		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate from %s", c.CACertFile)
		}

		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

// AllVariables combines secrets and variables with secrets taking priority.
func (c *Config) AllVariables() map[string]any {
	combined := make(map[string]any)

	maps.Copy(combined, c.Variables)
	maps.Copy(combined, c.Secrets)

	return combined
}

func (c *Config) Validate() error {
	if len(c.TestFiles) == 0 {
		return ErrNoTestFiles
	}

	for _, file := range c.TestFiles {
		if _, err := os.Stat(file); err != nil {
			return fmt.Errorf("test file %s not found: %w", file, err)
		}
	}

	if c.CACertFile != "" {
		if _, err := os.Stat(c.CACertFile); err != nil {
			return fmt.Errorf("CA certificate file %s not found: %w", c.CACertFile, err)
		}
	}

	return nil
}

type keyValueFlag struct {
	values        map[string]any
	invalidFormat error
	emptyName     error
}

func newKeyValueFlag(invalidFormat, emptyName error) *keyValueFlag {
	return &keyValueFlag{
		values:        make(map[string]any),
		invalidFormat: invalidFormat,
		emptyName:     emptyName,
	}
}

func (f *keyValueFlag) String() string {
	var pairs []string
	for k, v := range f.values {
		pairs = append(pairs, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(pairs, ",")
}

func (f *keyValueFlag) Set(value string) error {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("%w, got: %s", f.invalidFormat, value)
	}

	name := strings.TrimSpace(parts[0])
	if name == "" {
		return f.emptyName
	}

	f.values[name] = parts[1]
	return nil
}

func (f *keyValueFlag) Values() map[string]any {
	return f.values
}

func Parse(args []string) (*Config, *exit.Result) {
	if len(args) == 0 {
		return nil, exit.Errorf("Error: %v\n\n%s", ErrNoArguments, Usage())
	}

	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)

	// Suppress the default usage output since we handle it ourselves
	fs.Usage = func() {}
	// Suppress error output since we handle it ourselves
	fs.SetOutput(io.Discard)

	var (
		debug        = fs.Bool("debug", false, "Enable debug output showing request and response details")
		repeat       = fs.Int("repeat", 0, "Number of additional times to repeat test execution after the first run (negative for infinite loop)")
		insecure     = fs.Bool("insecure", false, "Skip TLS certificate verification")
		caCertFile   = fs.String("cacert", "", "Path to CA certificate file for TLS verification")
		secrets      = newKeyValueFlag(ErrInvalidSecretFormat, ErrEmptySecretName)
		secretFile   = fs.String("secret-file", "", "Path to key=value file containing secrets")
		variables    = newKeyValueFlag(ErrInvalidVariableFormat, ErrEmptyVariableName)
		variableFile = fs.String("variable-file", "", "Path to key=value file containing template variables")
		timeout      = fs.Duration("timeout", DefaultTimeout, "HTTP request timeout")
		rateLimit    = fs.Float64("rate-limit", 0, "Rate limit in requests per second (0 for unlimited)")
		output       = fs.String("output", "text", "Output format: text or json")
		secretSalt   = fs.String("secret-salt", timeNow().Format("2006-01-02"), "Salt to use for secret redaction hashes (default: current date)")
	)

	fs.Var(secrets, "secret", "Secret in format name=value (can be used multiple times)")
	fs.Var(variables, "variable", "Variable in format name=value (can be used multiple times)")

	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return nil, exit.Success(Usage())
		}
		return nil, exit.Errorf("Error: failed to parse arguments: %v\n\n%s", err, Usage())
	}

	// Get remaining positional arguments as test files
	files := fs.Args()
	if len(files) == 0 {
		return nil, exit.Errorf("Error: %v\n\n%s", ErrNoTestFiles, Usage())
	}

	finalVariables, err := mergeVariables(*variableFile, variables.Values())
	if err != nil {
		return nil, exit.Errorf("Error: failed to load variable file: %v\n\n%s", err, Usage())
	}

	finalSecrets, err := mergeSecrets(*secretFile, secrets.Values())
	if err != nil {
		return nil, exit.Errorf("Error: failed to load secret file: %v\n\n%s", err, Usage())
	}

	outputFormat, err := parseOutputFormat(*output)
	if err != nil {
		return nil, exit.Errorf("Error: %v\n\n%s", err, Usage())
	}

	config := &Config{
		TestFiles:      files,
		Debug:          *debug,
		Repeat:         *repeat,
		Insecure:       *insecure,
		CACertFile:     *caCertFile,
		RequestTimeout: *timeout,
		RateLimit:      *rateLimit,
		OutputFormat:   outputFormat,
		Secrets:        finalSecrets,
		SecretFile:     *secretFile,
		Variables:      finalVariables,
		SecretSalt:     *secretSalt,
	}

	if err := config.Validate(); err != nil {
		return nil, exit.Errorf("Error: %v\n\n%s", err, Usage())
	}

	return config, nil
}

func mergeVariables(variableFile string, cliVariables map[string]any) (map[string]any, error) {
	var merged map[string]any

	if variableFile != "" {
		fileVariables, err := loadKeyValueFile(variableFile)
		if err != nil {
			return nil, err
		}
		merged = make(map[string]any)
		maps.Copy(merged, fileVariables)
	}

	if len(cliVariables) > 0 {
		if merged == nil {
			merged = make(map[string]any)
		}
		maps.Copy(merged, cliVariables)
	}

	return merged, nil
}

func mergeSecrets(secretFile string, cliSecrets map[string]any) (map[string]any, error) {
	merged := make(map[string]any)

	if secretFile != "" {
		fileSecrets, err := loadKeyValueFile(secretFile)
		if err != nil {
			return nil, err
		}
		maps.Copy(merged, fileSecrets)
	}

	maps.Copy(merged, cliSecrets)
	return merged, nil
}

func parseOutputFormat(input string) (results.OutputFormat, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "text", "":
		return results.FormatText, nil
	case "json":
		return results.FormatJSON, nil
	default:
		return results.FormatText, fmt.Errorf("%w, got: %s", ErrInvalidOutputFormat, input)
	}
}

// loadVariableFile loads variables from key=value format with comment support.
func loadVariableFile(filename string) (map[string]any, error) {
	return loadKeyValueFile(filename)
}

func loadKeyValueFile(filename string) (map[string]any, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	variables := make(map[string]any)
	lines := strings.Split(string(data), "\n")

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format at line %d: %s (expected key=value)", lineNum+1, line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" {
			return nil, fmt.Errorf("empty key at line %d: %s", lineNum+1, line)
		}

		variables[key] = value
	}

	return variables, nil
}

func Usage() string {
	return `rq - HTTP testing tool

Usage: rq [options] <file1> [file2] ...

Options:
  --debug                 Enable debug output showing request and response details
  --repeat N              Number of additional times to repeat after first run (negative for infinite)
  --insecure              Skip TLS certificate verification
  --cacert FILE           Path to CA certificate file for TLS verification
  --timeout DURATION      HTTP request timeout (default: 30s)
  --rate-limit N          Rate limit in requests per second (0 for unlimited)
  --output FORMAT         Output format: text or json (default: text)
  --secret NAME=VALUE     Secret in format name=value (can be used multiple times)
  --secret-file FILE      Path to key=value file containing secrets
  --secret-salt SALT      Salt to use for secret redaction hashes (default: current date)
  --variable NAME=VALUE   Variable in format name=value (can be used multiple times)
  --variable-file FILE    Path to key=value file containing template variables
  -h, --help              Show this help message
  -v, --version           Show version information

Examples:
  rq test.yaml                           # Run test file once
  rq test.yaml --debug                   # Run with debug output
  rq test.yaml --rate-limit 5            # Rate limit to 5 requests per second
  rq test.yaml --repeat 1                # Run test file twice (1 + 1 additional)
  rq test.yaml --repeat -1               # Run test file infinitely
  rq file1.yaml file2.yaml              # Run multiple test files in sequence
  rq test.yaml --secret API_KEY=secret   # Pass secret to test
  rq test.yaml --variable HOST=localhost # Pass variable to test`
}

func (c *Config) HTTPClient() (*http.Client, error) {
	tlsConfig, err := c.TLSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS configuration: %w", err)
	}

	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy:                  http.ProxyFromEnvironment,
		DialContext:            dialer.DialContext,
		TLSClientConfig:        tlsConfig,
		TLSHandshakeTimeout:    10 * time.Second,
		ResponseHeaderTimeout:  10 * time.Second,
		ExpectContinueTimeout:  1 * time.Second,
		IdleConnTimeout:        60 * time.Second,
		MaxIdleConns:           100,
		MaxIdleConnsPerHost:    10,
		MaxConnsPerHost:        50,
		MaxResponseHeaderBytes: 1 << 20, // 1 MiB
	}

	client := &http.Client{
		Timeout:   c.RequestTimeout,
		Transport: transport,
	}

	return client, nil
}
