package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jacoelho/rq/internal/exit"
)

const (
	// DefaultTimeout is the default timeout for HTTP requests.
	DefaultTimeout = 30 * time.Second
)

var (
	ErrNoArguments           = errors.New("no arguments provided")
	ErrNoTestFiles           = errors.New("no test files specified")
	ErrInvalidSecretFormat   = errors.New("secret must be in format name=value")
	ErrEmptySecretName       = errors.New("secret name cannot be empty")
	ErrInvalidVariableFormat = errors.New("variable must be in format name=value")
	ErrEmptyVariableName     = errors.New("variable name cannot be empty")
)

// Config represents the complete configuration for the rq tool.
type Config struct {
	// Test execution
	TestFiles []string
	Debug     bool
	Repeat    int // Additional iterations after first run (negative = infinite)

	// HTTP client configuration
	Insecure       bool
	CACertFile     string
	RequestTimeout time.Duration
	RateLimit      float64 // Requests per second (0 = unlimited)

	// Template variables
	Secrets    map[string]any
	SecretFile string
	Variables  map[string]any
}

// TLSConfig returns a TLS configuration based on the config settings.
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

// AllVariables returns a combined map of secrets and variables for template substitution.
// Secrets take priority over variables when keys conflict.
func (c *Config) AllVariables() map[string]any {
	combined := make(map[string]any)

	maps.Copy(combined, c.Variables)
	maps.Copy(combined, c.Secrets)

	return combined
}

// Validate validates the configuration and returns an error if invalid.
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

// secretsFlag implements flag.Value for parsing multiple -secret flags.
type secretsFlag map[string]any

// String returns a string representation of the secrets flag for flag.Value interface.
func (s secretsFlag) String() string {
	var pairs []string
	for k, v := range s {
		pairs = append(pairs, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(pairs, ",")
}

// Set parses and stores a secret in name=value format for flag.Value interface.
func (s secretsFlag) Set(value string) error {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("%w, got: %s", ErrInvalidSecretFormat, value)
	}

	name := strings.TrimSpace(parts[0])
	if name == "" {
		return ErrEmptySecretName
	}

	s[name] = parts[1]
	return nil
}

// variablesFlag implements flag.Value for parsing multiple -variable flags.
type variablesFlag map[string]any

// String returns a string representation of the variables flag for flag.Value interface.
func (v variablesFlag) String() string {
	var pairs []string
	for k, val := range v {
		pairs = append(pairs, fmt.Sprintf("%s=%v", k, val))
	}
	return strings.Join(pairs, ",")
}

// Set parses and stores a variable in name=value format for flag.Value interface.
func (v variablesFlag) Set(value string) error {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("%w, got: %s", ErrInvalidVariableFormat, value)
	}

	name := strings.TrimSpace(parts[0])
	if name == "" {
		return ErrEmptyVariableName
	}

	v[name] = parts[1]
	return nil
}

// Parse parses command-line arguments and returns a validated Config.
// If parsing fails or help is requested, returns nil config and exit result.
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
		secrets      = make(secretsFlag)
		secretFile   = fs.String("secret-file", "", "Path to key=value file containing secrets")
		variables    = make(variablesFlag)
		variableFile = fs.String("variable-file", "", "Path to key=value file containing template variables")
		timeout      = fs.Duration("timeout", DefaultTimeout, "HTTP request timeout")
		rateLimit    = fs.Float64("rate-limit", 0, "Rate limit in requests per second (0 for unlimited)")
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

	// Load variables with proper precedence: file variables first, then command-line variables
	var finalVariables map[string]any
	if *variableFile != "" {
		fileVariables, err := loadVariableFile(*variableFile)
		if err != nil {
			return nil, exit.Errorf("Error: failed to load variable file: %v\n\n%s", err, Usage())
		}
		finalVariables = make(map[string]any)
		maps.Copy(finalVariables, fileVariables)
	}

	// Command-line variables take precedence over file variables
	if len(variables) > 0 {
		if finalVariables == nil {
			finalVariables = make(map[string]any)
		}
		maps.Copy(finalVariables, variables)
	}

	finalSecrets := make(map[string]any)
	if *secretFile != "" {
		fileSecrets, err := loadVariableFile(*secretFile)
		if err != nil {
			return nil, exit.Errorf("Error: failed to load secret file: %v\n\n%s", err, Usage())
		}
		maps.Copy(finalSecrets, fileSecrets)
	}
	maps.Copy(finalSecrets, secrets)

	config := &Config{
		TestFiles:      files,
		Debug:          *debug,
		Repeat:         *repeat,
		Insecure:       *insecure,
		CACertFile:     *caCertFile,
		RequestTimeout: *timeout,
		RateLimit:      *rateLimit,
		Secrets:        finalSecrets,
		SecretFile:     *secretFile,
		Variables:      finalVariables,
	}

	if err := config.Validate(); err != nil {
		return nil, exit.Errorf("Error: %v\n\n%s", err, Usage())
	}

	return config, nil
}

// loadVariableFile loads variables from a key=value format file.
// It supports comments (lines starting with #) and empty lines.
// Returns an error if the file format is invalid or the file cannot be read.
func loadVariableFile(filename string) (map[string]any, error) {
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

// Usage returns a usage string for the CLI tool.
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
  --secret NAME=VALUE     Secret in format name=value (can be used multiple times)
  --secret-file FILE      Path to key=value file containing secrets
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

// HTTPClient creates an HTTP client configured with the settings from this Config.
func (c *Config) HTTPClient() (*http.Client, error) {
	tlsConfig, err := c.TLSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS configuration: %w", err)
	}

	return &http.Client{
		Timeout: c.RequestTimeout,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}, nil
}
