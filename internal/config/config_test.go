package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// generateTestCertificate creates a self-signed certificate for testing purposes
func generateTestCertificate() ([]byte, error) {
	// Generate a private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Country:      []string{"AU"},
			Province:     []string{"Some-State"},
			Organization: []string{"Some Organization"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // Valid for 1 year
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Create the certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Encode certificate to PEM format
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	return certPEM, nil
}

func TestParse(t *testing.T) {
	tempDir := t.TempDir()
	testFile1 := filepath.Join(tempDir, "test1.yaml")
	testFile2 := filepath.Join(tempDir, "test2.yaml")
	varsFile := filepath.Join(tempDir, "vars.env")
	secretsFile := filepath.Join(tempDir, "secrets.env")

	if err := os.WriteFile(testFile1, []byte("test: content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile2, []byte("test: content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(varsFile, []byte("var1=value1\nvar2=value2"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secretsFile, []byte("secret1=value1\nsecret2=value2"), 0644); err != nil {
		t.Fatal(err)
	}

	caCertFile := filepath.Join(tempDir, "ca.pem")
	if err := os.WriteFile(caCertFile, []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		args    []string
		want    *Config
		wantErr bool
	}{
		{
			name: "valid_single_file",
			args: []string{"rq", testFile1},
			want: &Config{
				TestFiles:      []string{testFile1},
				Repeat:         0,
				Insecure:       false,
				CACertFile:     "",
				RequestTimeout: DefaultTimeout,
				RateLimit:      0,
				Secrets:        map[string]any{},
				Variables:      nil,
			},
			wantErr: false,
		},
		{
			name: "valid_multiple_files",
			args: []string{"rq", testFile1, testFile2},
			want: &Config{
				TestFiles:      []string{testFile1, testFile2},
				Repeat:         0,
				Insecure:       false,
				CACertFile:     "",
				RequestTimeout: DefaultTimeout,
				RateLimit:      0,
				Secrets:        map[string]any{},
				Variables:      nil,
			},
			wantErr: false,
		},
		{
			name: "with_insecure_flag",
			args: []string{"rq", "--insecure", testFile1},
			want: &Config{
				TestFiles:      []string{testFile1},
				Repeat:         0,
				Insecure:       true,
				CACertFile:     "",
				RequestTimeout: DefaultTimeout,
				RateLimit:      0,
				Secrets:        map[string]any{},
				Variables:      nil,
			},
			wantErr: false,
		},
		{
			name: "with_cacert",
			args: []string{"rq", "--cacert", caCertFile, testFile1},
			want: &Config{
				TestFiles:      []string{testFile1},
				Repeat:         0,
				Insecure:       false,
				CACertFile:     caCertFile,
				RequestTimeout: DefaultTimeout,
				RateLimit:      0,
				Secrets:        map[string]any{},
				Variables:      nil,
			},
			wantErr: false,
		},
		{
			name: "with_timeout",
			args: []string{"rq", "--timeout", "10s", testFile1},
			want: &Config{
				TestFiles:      []string{testFile1},
				Repeat:         0,
				Insecure:       false,
				CACertFile:     "",
				RequestTimeout: 10 * time.Second,
				RateLimit:      0,
				Secrets:        map[string]any{},
				Variables:      nil,
			},
			wantErr: false,
		},
		{
			name: "with_secrets",
			args: []string{"rq", "--secret", "key1=value1", "--secret", "key2=value2", testFile1},
			want: &Config{
				TestFiles:      []string{testFile1},
				Repeat:         0,
				Insecure:       false,
				CACertFile:     "",
				RequestTimeout: DefaultTimeout,
				RateLimit:      0,
				Secrets:        map[string]any{"key1": "value1", "key2": "value2"},
				Variables:      nil,
			},
			wantErr: false,
		},
		{
			name: "with_variable_file",
			args: []string{"rq", "--variable-file", varsFile, testFile1},
			want: &Config{
				TestFiles:      []string{testFile1},
				Repeat:         0,
				Insecure:       false,
				CACertFile:     "",
				RequestTimeout: DefaultTimeout,
				RateLimit:      0,
				Secrets:        map[string]any{},
				Variables:      map[string]any{"var1": "value1", "var2": "value2"},
			},
			wantErr: false,
		},
		{
			name: "with_secret_file",
			args: []string{"rq", "--secret-file", secretsFile, testFile1},
			want: &Config{
				TestFiles:      []string{testFile1},
				Repeat:         0,
				Insecure:       false,
				CACertFile:     "",
				RequestTimeout: DefaultTimeout,
				RateLimit:      0,
				Secrets:        map[string]any{"secret1": "value1", "secret2": "value2"},
				SecretFile:     secretsFile,
				Variables:      nil,
			},
			wantErr: false,
		},
		{
			name: "with_variables",
			args: []string{"rq", "--variable", "key1=value1", "--variable", "key2=value2", testFile1},
			want: &Config{
				TestFiles:      []string{testFile1},
				Repeat:         0,
				Insecure:       false,
				CACertFile:     "",
				RequestTimeout: DefaultTimeout,
				RateLimit:      0,
				Secrets:        map[string]any{},
				Variables:      map[string]any{"key1": "value1", "key2": "value2"},
			},
			wantErr: false,
		},
		{
			name: "with_variable_file_and_variables",
			args: []string{"rq", "--variable-file", varsFile, "--variable", "var1=override", "--variable", "var3=new", testFile1},
			want: &Config{
				TestFiles:      []string{testFile1},
				Repeat:         0,
				Insecure:       false,
				CACertFile:     "",
				RequestTimeout: DefaultTimeout,
				RateLimit:      0,
				Secrets:        map[string]any{},
				Variables:      map[string]any{"var1": "override", "var2": "value2", "var3": "new"},
			},
			wantErr: false,
		},
		{
			name: "with_rate_limit",
			args: []string{"rq", "--rate-limit", "10", testFile1},
			want: &Config{
				TestFiles:      []string{testFile1},
				Repeat:         0,
				Insecure:       false,
				CACertFile:     "",
				RequestTimeout: DefaultTimeout,
				RateLimit:      10,
				Secrets:        map[string]any{},
				Variables:      nil,
			},
			wantErr: false,
		},
		{
			name: "with_fractional_rate_limit",
			args: []string{"rq", "--rate-limit", "0.5", testFile1},
			want: &Config{
				TestFiles:      []string{testFile1},
				Repeat:         0,
				Insecure:       false,
				CACertFile:     "",
				RequestTimeout: DefaultTimeout,
				RateLimit:      0.5,
				Secrets:        map[string]any{},
				Variables:      nil,
			},
			wantErr: false,
		},
		{
			name: "with_zero_rate_limit",
			args: []string{"rq", "--rate-limit", "0", testFile1},
			want: &Config{
				TestFiles:      []string{testFile1},
				Repeat:         0,
				Insecure:       false,
				CACertFile:     "",
				RequestTimeout: DefaultTimeout,
				RateLimit:      0,
				Secrets:        map[string]any{},
				Variables:      nil,
			},
			wantErr: false,
		},
		{
			name:    "no_arguments",
			args:    []string{},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "missing_test_flag",
			args:    []string{"rq"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty_test_flag",
			args:    []string{"rq"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid_timeout",
			args:    []string{"rq", "--timeout", "invalid", testFile1},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "nonexistent_variable_file",
			args:    []string{"rq", "--variable-file", "/nonexistent/vars.env", testFile1},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid_secret_format",
			args:    []string{"rq", "--secret", "invalid", testFile1},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty_secret_name",
			args:    []string{"rq", "--secret", "=value", testFile1},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "nonexistent_secret_file",
			args:    []string{"rq", "--secret-file", "/nonexistent/secrets.env", testFile1},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "help_flag",
			args:    []string{"rq", "-help"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid_variable_format",
			args:    []string{"rq", "--variable", "invalid", testFile1},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty_variable_name",
			args:    []string{"rq", "--variable", "=value", testFile1},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid_rate_limit",
			args:    []string{"rq", "--rate-limit", "invalid", testFile1},
			want:    nil,
			wantErr: true,
		},
		{
			name: "with_repeat_flag",
			args: []string{"rq", "--repeat", "3", testFile1},
			want: &Config{
				TestFiles:      []string{testFile1},
				Repeat:         3,
				Insecure:       false,
				CACertFile:     "",
				RequestTimeout: DefaultTimeout,
				RateLimit:      0,
				Secrets:        map[string]any{},
				Variables:      nil,
			},
			wantErr: false,
		},
		{
			name: "with_infinite_repeat",
			args: []string{"rq", "--repeat", "-1", testFile1},
			want: &Config{
				TestFiles:      []string{testFile1},
				Repeat:         -1,
				Insecure:       false,
				CACertFile:     "",
				RequestTimeout: DefaultTimeout,
				RateLimit:      0,
				Secrets:        map[string]any{},
				Variables:      nil,
			},
			wantErr: false,
		},
		{
			name: "valid_repeat_zero",
			args: []string{"rq", "--repeat", "0", testFile1},
			want: &Config{
				TestFiles:      []string{testFile1},
				Repeat:         0,
				Insecure:       false,
				CACertFile:     "",
				RequestTimeout: DefaultTimeout,
				RateLimit:      0,
				Secrets:        map[string]any{},
				Variables:      nil,
			},
			wantErr: false,
		},
		{
			name: "valid_repeat_negative",
			args: []string{"rq", "--repeat", "-2", testFile1},
			want: &Config{
				TestFiles:      []string{testFile1},
				Repeat:         -2,
				Insecure:       false,
				CACertFile:     "",
				RequestTimeout: DefaultTimeout,
				RateLimit:      0,
				Secrets:        map[string]any{},
				Variables:      nil,
			},
			wantErr: false,
		},
		{
			name:    "invalid_repeat_format",
			args:    []string{"rq", "--repeat", "invalid", testFile1},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, exitResult := Parse(tt.args)

			if tt.wantErr {
				if exitResult == nil {
					t.Errorf("Parse() expected error but got none")
					return
				}
				// For help flag, expect exit code 0, for errors expect exit code 1
				if tt.name == "help_flag" && exitResult.ExitCode != 0 {
					t.Errorf("Parse() help flag should have exit code 0, got %d", exitResult.ExitCode)
				} else if tt.name != "help_flag" && exitResult.ExitCode != 1 {
					t.Errorf("Parse() error should have exit code 1, got %d", exitResult.ExitCode)
				}
				return
			}

			if exitResult != nil {
				t.Errorf("Parse() unexpected error: exit code %d, message: %s", exitResult.ExitCode, exitResult.Message)
				return
			}

			if !reflect.DeepEqual(cfg, tt.want) {
				t.Errorf("Parse() = %v, want %v", cfg, tt.want)
			}
		})
	}
}

func TestParseHelpFlag(t *testing.T) {
	_, exitResult := Parse([]string{"rq", "-help"})
	if exitResult == nil {
		t.Fatal("expected exit result for help flag")
	}
	if exitResult.ExitCode != 0 {
		t.Errorf("expected exit code 0 for help, got %d", exitResult.ExitCode)
	}

	_, exitResult = Parse([]string{"rq", "--help"})
	if exitResult == nil {
		t.Fatal("expected exit result for --help flag")
	}
	if exitResult.ExitCode != 0 {
		t.Errorf("expected exit code 0 for --help, got %d", exitResult.ExitCode)
	}
}

func TestSecretsFlag(t *testing.T) {
	tests := []struct {
		name    string
		values  []string
		want    map[string]any
		wantErr bool
	}{
		{
			name:   "empty",
			values: []string{},
			want:   map[string]any{},
		},
		{
			name:    "invalid format - no equals",
			values:  []string{"invalid"},
			wantErr: true,
		},
		{
			name:    "invalid format - empty name",
			values:  []string{"=value"},
			wantErr: true,
		},
		{
			name:   "single secret",
			values: []string{"key=value"},
			want:   map[string]any{"key": "value"},
		},
		{
			name:    "invalid format - empty value allowed",
			values:  []string{"key="},
			want:    map[string]any{"key": ""},
			wantErr: false,
		},
		{
			name:    "invalid format - multiple equals",
			values:  []string{"key=value=extra"},
			want:    map[string]any{"key": "value=extra"},
			wantErr: false,
		},
		{
			name:   "multiple secrets",
			values: []string{"key1=value1", "key2=value2"},
			want:   map[string]any{"key1": "value1", "key2": "value2"},
		},
		{
			name:   "single secret",
			values: []string{"api_key=secret123"},
			want:   map[string]any{"api_key": "secret123"},
		},
		{
			name:   "multiple secrets",
			values: []string{"api_key=secret123", "user=admin", "token=xyz"},
			want:   map[string]any{"api_key": "secret123", "user": "admin", "token": "xyz"},
		},
		{
			name:   "secret with base64 value",
			values: []string{"base64=dGVzdD0xMjM="},
			want:   map[string]any{"base64": "dGVzdD0xMjM="},
		},
		{
			name:   "secret with empty value",
			values: []string{"api_key=secret123", "empty="},
			want:   map[string]any{"api_key": "secret123", "empty": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secrets := make(secretsFlag)
			for _, value := range tt.values {
				err := secrets.Set(value)
				if (err != nil) != tt.wantErr {
					t.Errorf("secretsFlag.Set() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
			}

			if !tt.wantErr && !reflect.DeepEqual(map[string]any(secrets), tt.want) {
				t.Errorf("secretsFlag = %v, want %v", secrets, tt.want)
			}
		})
	}
}

func TestLoadVariableFile(t *testing.T) {
	tempDir := t.TempDir()

	envFile := filepath.Join(tempDir, "vars.env")
	envContent := `api_url=https://api.example.com
version=v1
debug=true
port=8080`
	if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
		t.Fatalf("Failed to create env file: %v", err)
	}

	envFile2 := filepath.Join(tempDir, "vars2.env")
	envContent2 := `api_url=https://api.example.com
version=v2
debug=false
port=9090`
	if err := os.WriteFile(envFile2, []byte(envContent2), 0644); err != nil {
		t.Fatalf("Failed to create second env file: %v", err)
	}

	envFile3 := filepath.Join(tempDir, "vars3.env")
	envContent3 := `# This is a comment
api_url=https://api.example.com

# Another comment
version=v3
# debug=true  

timeout=30s`
	if err := os.WriteFile(envFile3, []byte(envContent3), 0644); err != nil {
		t.Fatalf("Failed to create third env file: %v", err)
	}

	invalidFile := filepath.Join(tempDir, "invalid.env")
	invalidContent := `invalid format without equals
key_without_value
=value_without_key`
	if err := os.WriteFile(invalidFile, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to create invalid file: %v", err)
	}

	tests := []struct {
		name     string
		filename string
		want     map[string]any
		wantErr  bool
	}{
		{
			name:     "valid_env_file",
			filename: envFile,
			want: map[string]any{
				"api_url": "https://api.example.com",
				"version": "v1",
				"debug":   "true",
				"port":    "8080",
			},
			wantErr: false,
		},
		{
			name:     "valid_env_file_2",
			filename: envFile2,
			want: map[string]any{
				"api_url": "https://api.example.com",
				"version": "v2",
				"debug":   "false",
				"port":    "9090",
			},
			wantErr: false,
		},
		{
			name:     "env_file_with_comments",
			filename: envFile3,
			want: map[string]any{
				"api_url": "https://api.example.com",
				"version": "v3",
				"timeout": "30s",
			},
			wantErr: false,
		},
		{
			name:     "nonexistent_file",
			filename: "/nonexistent/file.env",
			want:     nil,
			wantErr:  true,
		},
		{
			name:     "invalid_file_content",
			filename: invalidFile,
			want:     nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadVariableFile(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadVariableFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Check each key-value pair individually to avoid map ordering issues
				if len(got) != len(tt.want) {
					t.Errorf("loadVariableFile() length = %d, want %d", len(got), len(tt.want))
					return
				}
				for key, expectedValue := range tt.want {
					if gotValue, exists := got[key]; !exists {
						t.Errorf("loadVariableFile() missing key %s", key)
					} else if !reflect.DeepEqual(gotValue, expectedValue) {
						t.Errorf("loadVariableFile() key %s = %v, want %v", key, gotValue, expectedValue)
					}
				}
			}
		})
	}
}

func TestConfig_AllVariables(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   map[string]any
	}{
		{
			name: "empty",
			config: Config{
				Secrets:   map[string]any{},
				Variables: nil,
			},
			want: map[string]any{},
		},
		{
			name: "only_secrets",
			config: Config{
				Secrets:   map[string]any{"secret1": "value1", "secret2": "value2"},
				Variables: nil,
			},
			want: map[string]any{"secret1": "value1", "secret2": "value2"},
		},
		{
			name: "secrets_override_variables",
			config: Config{
				Secrets:   map[string]any{"secret1": "value1", "shared": "from_secret"},
				Variables: map[string]any{"var1": "value1", "shared": "from_variable"},
			},
			want: map[string]any{"var1": "value1", "secret1": "value1", "shared": "from_secret"},
		},
		{
			name: "only_variables",
			config: Config{
				Secrets:   map[string]any{},
				Variables: map[string]any{"var1": "value1", "var2": "value2"},
			},
			want: map[string]any{"var1": "value1", "var2": "value2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.AllVariables()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Config.AllVariables() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_TLSConfig(t *testing.T) {
	tempDir := t.TempDir()

	validCACert := filepath.Join(tempDir, "ca.pem")
	validCertContent, err := generateTestCertificate()
	if err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}
	if err := os.WriteFile(validCACert, validCertContent, 0644); err != nil {
		t.Fatalf("Failed to create valid CA cert file: %v", err)
	}

	invalidCACert := filepath.Join(tempDir, "invalid_ca.pem")
	invalidCertContent := `-----BEGIN CERTIFICATE-----
invalid certificate content
-----END CERTIFICATE-----`
	if err := os.WriteFile(invalidCACert, []byte(invalidCertContent), 0644); err != nil {
		t.Fatalf("Failed to create invalid CA cert file: %v", err)
	}

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		checkFn func(*testing.T, *tls.Config)
	}{
		{
			name: "default_config",
			config: &Config{
				Insecure:   false,
				CACertFile: "",
			},
			wantErr: false,
			checkFn: func(t *testing.T, tlsConfig *tls.Config) {
				if tlsConfig.InsecureSkipVerify {
					t.Error("Expected InsecureSkipVerify to be false")
				}
				if tlsConfig.RootCAs != nil {
					t.Error("Expected RootCAs to be nil")
				}
			},
		},
		{
			name: "insecure_config",
			config: &Config{
				Insecure:   true,
				CACertFile: "",
			},
			wantErr: false,
			checkFn: func(t *testing.T, tlsConfig *tls.Config) {
				if !tlsConfig.InsecureSkipVerify {
					t.Error("Expected InsecureSkipVerify to be true")
				}
			},
		},
		{
			name: "with_nonexistent_ca_cert",
			config: &Config{
				Insecure:   false,
				CACertFile: "/nonexistent/ca.pem",
			},
			wantErr: true,
			checkFn: nil,
		},
		{
			name: "with_invalid_ca_cert",
			config: &Config{
				Insecure:   false,
				CACertFile: invalidCACert,
			},
			wantErr: true,
			checkFn: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlsConfig, err := tt.config.TLSConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.TLSConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.checkFn != nil {
				tt.checkFn(t, tlsConfig)
			}
		})
	}
}

func TestUsage(t *testing.T) {
	usage := Usage()
	if usage == "" {
		t.Error("Usage() returned empty string")
	}

	expectedSections := []string{
		"rq - HTTP testing tool",
		"Usage: rq [options]",
		"Options:",
		"--help",
		"--debug",
		"--rate-limit",
		"--repeat",
		"Examples:",
	}

	for _, section := range expectedSections {
		if !strings.Contains(usage, section) {
			t.Errorf("Usage() missing expected section: %s", section)
		}
	}
}

func TestConfig_HTTPClient(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantErr   bool
		checkFunc func(*http.Client) error
	}{
		{
			name: "basic_http_client",
			config: &Config{
				RequestTimeout: 10 * time.Second,
				Insecure:       false,
			},
			wantErr: false,
			checkFunc: func(client *http.Client) error {
				if client.Timeout != 10*time.Second {
					return fmt.Errorf("expected timeout 10s, got %v", client.Timeout)
				}
				return nil
			},
		},
		{
			name: "insecure_http_client",
			config: &Config{
				RequestTimeout: 30 * time.Second,
				Insecure:       true,
			},
			wantErr: false,
			checkFunc: func(client *http.Client) error {
				transport, ok := client.Transport.(*http.Transport)
				if !ok {
					return fmt.Errorf("expected *http.Transport, got %T", client.Transport)
				}
				if !transport.TLSClientConfig.InsecureSkipVerify {
					return fmt.Errorf("expected InsecureSkipVerify to be true")
				}
				return nil
			},
		},
		{
			name: "with_invalid_cacert",
			config: &Config{
				RequestTimeout: 30 * time.Second,
				CACertFile:     "/nonexistent/ca.pem",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := tt.config.HTTPClient()

			if tt.wantErr {
				if err == nil {
					t.Error("HTTPClient() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("HTTPClient() unexpected error: %v", err)
				return
			}

			if client == nil {
				t.Error("HTTPClient() returned nil client")
				return
			}

			if tt.checkFunc != nil {
				if err := tt.checkFunc(client); err != nil {
					t.Errorf("HTTPClient() validation failed: %v", err)
				}
			}
		})
	}
}

func TestVariablesFlag(t *testing.T) {
	tests := []struct {
		name    string
		values  []string
		want    map[string]any
		wantErr bool
	}{
		{
			name:   "empty",
			values: []string{},
			want:   map[string]any{},
		},
		{
			name:    "invalid format - no equals",
			values:  []string{"invalid"},
			wantErr: true,
		},
		{
			name:    "invalid format - empty name",
			values:  []string{"=value"},
			wantErr: true,
		},
		{
			name:   "single variable",
			values: []string{"key=value"},
			want:   map[string]any{"key": "value"},
		},
		{
			name:    "empty value allowed",
			values:  []string{"key="},
			want:    map[string]any{"key": ""},
			wantErr: false,
		},
		{
			name:    "multiple equals",
			values:  []string{"key=value=extra"},
			want:    map[string]any{"key": "value=extra"},
			wantErr: false,
		},
		{
			name:   "multiple variables",
			values: []string{"key1=value1", "key2=value2"},
			want:   map[string]any{"key1": "value1", "key2": "value2"},
		},
		{
			name:   "variable with special characters",
			values: []string{"host=localhost:8080", "path=/api/v1"},
			want:   map[string]any{"host": "localhost:8080", "path": "/api/v1"},
		},
		{
			name:   "variable with empty value",
			values: []string{"host=localhost", "empty="},
			want:   map[string]any{"host": "localhost", "empty": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			variables := make(variablesFlag)
			for _, value := range tt.values {
				err := variables.Set(value)
				if (err != nil) != tt.wantErr {
					t.Errorf("variablesFlag.Set() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
			}

			if !tt.wantErr && !reflect.DeepEqual(map[string]any(variables), tt.want) {
				t.Errorf("variablesFlag = %v, want %v", variables, tt.want)
			}
		})
	}
}
