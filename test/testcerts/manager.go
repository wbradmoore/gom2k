package testcerts

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCertificates manages temporary SSL certificates for testing
type TestCertificates struct {
	tempDir          string
	keystorePath     string
	truststorePath   string
	serverCertPath   string
	serverKeyPath    string
	caPath           string
	password         string
	t                *testing.T
}

// CertificateOptions configures certificate generation
type CertificateOptions struct {
	Hosts           []string
	Password        string
	ValidityHours   int
	GenerateInvalid bool
}

// DefaultOptions returns default certificate options
func DefaultOptions() *CertificateOptions {
	return &CertificateOptions{
		Hosts:           []string{"localhost", "127.0.0.1"},
		Password:        "testpass",
		ValidityHours:   24,
		GenerateInvalid: false,
	}
}

// CreateTestCertificates creates temporary SSL certificates for testing
func CreateTestCertificates(t *testing.T, opts *CertificateOptions) (*TestCertificates, error) {
	if opts == nil {
		opts = DefaultOptions()
	}

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "gom2k-test-certs-*")
	if err != nil {
		return nil, err
	}

	tc := &TestCertificates{
		tempDir:          tempDir,
		keystorePath:     filepath.Join(tempDir, "kafka.keystore.jks"),
		truststorePath:   filepath.Join(tempDir, "kafka.truststore.jks"),
		serverCertPath:   filepath.Join(tempDir, "server.crt"),
		serverKeyPath:    filepath.Join(tempDir, "server.key"),
		caPath:           filepath.Join(tempDir, "ca.crt"),
		password:         opts.Password,
		t:                t,
	}

	// Register cleanup
	t.Cleanup(func() {
		tc.Cleanup()
	})

	// Generate certificates
	if err := tc.generateCertificates(opts); err != nil {
		tc.Cleanup()
		return nil, err
	}

	return tc, nil
}

// GetKeystorePath returns the path to the Java keystore
func (tc *TestCertificates) GetKeystorePath() string {
	return tc.keystorePath
}

// GetTruststorePath returns the path to the Java truststore
func (tc *TestCertificates) GetTruststorePath() string {
	return tc.truststorePath
}

// GetServerCertPath returns the path to the server certificate
func (tc *TestCertificates) GetServerCertPath() string {
	return tc.serverCertPath
}

// GetServerKeyPath returns the path to the server private key
func (tc *TestCertificates) GetServerKeyPath() string {
	return tc.serverKeyPath
}

// GetCAPath returns the path to the CA certificate
func (tc *TestCertificates) GetCAPath() string {
	return tc.caPath
}

// GetPassword returns the certificate password
func (tc *TestCertificates) GetPassword() string {
	return tc.password
}

// GetTempDir returns the temporary directory path
func (tc *TestCertificates) GetTempDir() string {
	return tc.tempDir
}

// Exists checks if all certificate files exist
func (tc *TestCertificates) Exists() bool {
	paths := []string{
		tc.keystorePath,
		tc.truststorePath,
		tc.serverCertPath,
		tc.serverKeyPath,
		tc.caPath,
	}

	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return false
		}
	}

	return true
}

// Cleanup removes the temporary directory and all certificates
func (tc *TestCertificates) Cleanup() {
	if tc.tempDir != "" {
		os.RemoveAll(tc.tempDir)
	}
}