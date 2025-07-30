package testcerts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// generateCertificates creates SSL certificates using system mkcert and converts them to required formats
func (tc *TestCertificates) generateCertificates(opts *CertificateOptions) error {
	// Check if mkcert is available on system
	if err := tc.checkMkcertAvailable(); err != nil {
		return err
	}

	// Generate base certificates with mkcert
	if err := tc.generateMkcertCertificates(opts.Hosts); err != nil {
		return fmt.Errorf("failed to generate mkcert certificates: %w", err)
	}

	// Convert to Java formats for Kafka
	if err := tc.convertToJavaFormat(opts.Password); err != nil {
		return fmt.Errorf("failed to convert to Java format: %w", err)
	}

	// Generate invalid certificates if requested (for negative testing)
	if opts.GenerateInvalid {
		if err := tc.generateInvalidCertificates(); err != nil {
			return fmt.Errorf("failed to generate invalid certificates: %w", err)
		}
	}

	return nil
}

// checkMkcertAvailable verifies mkcert is installed and available on system
func (tc *TestCertificates) checkMkcertAvailable() error {
	_, err := exec.LookPath("mkcert")
	if err != nil {
		return fmt.Errorf("mkcert is required for test certificate generation but not found in PATH: %w\n" +
			"Install with:\n" +
			"  macOS: brew install mkcert\n" +
			"  Linux: https://github.com/FiloSottile/mkcert#linux\n" +
			"  Manual: https://github.com/FiloSottile/mkcert#installation", err)
	}
	return nil
}

// generateMkcertCertificates generates certificates using system mkcert
func (tc *TestCertificates) generateMkcertCertificates(hosts []string) error {
	// Change to temp directory for mkcert output
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	
	if err := os.Chdir(tc.tempDir); err != nil {
		return fmt.Errorf("failed to change to temp directory: %w", err)
	}
	defer os.Chdir(originalDir)

	// Generate certificate for specified hosts
	args := append([]string{}, hosts...)
	cmd := exec.Command("mkcert", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mkcert failed: %w\nOutput: %s", err, string(output))
	}

	// mkcert creates files like "localhost+1.pem" and "localhost+1-key.pem"
	// Find the generated files and rename them to standard names
	if err := tc.standardizeCertificateNames(hosts); err != nil {
		return fmt.Errorf("failed to standardize certificate names: %w", err)
	}

	// Copy CA certificate
	if err := tc.copyCACertificate(); err != nil {
		return fmt.Errorf("failed to copy CA certificate: %w", err)
	}

	return nil
}

// standardizeCertificateNames renames mkcert output to standard names
func (tc *TestCertificates) standardizeCertificateNames(hosts []string) error {
	// mkcert generates files like "localhost+1.pem" based on hosts
	// We need to find and rename them to "server.crt" and "server.key"
	
	// Find the generated certificate file (ends with .pem, not with -key.pem)
	files, err := filepath.Glob(filepath.Join(tc.tempDir, "*.pem"))
	if err != nil {
		return fmt.Errorf("failed to find generated certificates: %w", err)
	}

	var certFile, keyFile string
	for _, file := range files {
		if strings.HasSuffix(file, "-key.pem") {
			keyFile = file
		} else {
			certFile = file
		}
	}

	if certFile == "" || keyFile == "" {
		return fmt.Errorf("could not find generated certificate files in %s", tc.tempDir)
	}

	// Rename to standard names
	if err := os.Rename(certFile, tc.serverCertPath); err != nil {
		return fmt.Errorf("failed to rename certificate file: %w", err)
	}
	
	if err := os.Rename(keyFile, tc.serverKeyPath); err != nil {
		return fmt.Errorf("failed to rename key file: %w", err)
	}

	return nil
}

// copyCACertificate copies the mkcert CA certificate to our temp directory
func (tc *TestCertificates) copyCACertificate() error {
	// Get CA root path from mkcert
	cmd := exec.Command("mkcert", "-CAROOT")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get mkcert CA root: %w", err)
	}

	caRoot := strings.TrimSpace(string(output))
	caSourcePath := filepath.Join(caRoot, "rootCA.pem")

	// Copy CA certificate to our temp directory
	caData, err := os.ReadFile(caSourcePath)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate from %s: %w", caSourcePath, err)
	}

	if err := os.WriteFile(tc.caPath, caData, 0644); err != nil {
		return fmt.Errorf("failed to write CA certificate: %w", err)
	}

	return nil
}

// convertToJavaFormat converts PEM certificates to Java keystore/truststore format
func (tc *TestCertificates) convertToJavaFormat(password string) error {
	// Create PKCS12 intermediate format
	p12Path := filepath.Join(tc.tempDir, "server.p12")
	
	cmd := exec.Command("openssl", "pkcs12", "-export",
		"-in", tc.serverCertPath,
		"-inkey", tc.serverKeyPath,
		"-out", p12Path,
		"-name", "kafka-server",
		"-password", "pass:"+password)
	
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create PKCS12 keystore: %w\nOutput: %s", err, string(output))
	}

	// Convert PKCS12 to JKS keystore
	cmd = exec.Command("keytool", "-importkeystore",
		"-srckeystore", p12Path,
		"-srcstoretype", "PKCS12",
		"-srcstorepass", password,
		"-destkeystore", tc.keystorePath,
		"-deststoretype", "JKS",
		"-deststorepass", password,
		"-destkeypass", password)
	
	cmd.Stdout = nil // Suppress keytool output
	cmd.Stderr = nil
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create JKS keystore: %w", err)
	}

	// Create truststore with CA certificate
	cmd = exec.Command("keytool", "-importcert",
		"-alias", "ca",
		"-keystore", tc.truststorePath,
		"-storepass", password,
		"-file", tc.caPath,
		"-noprompt")
	
	cmd.Stdout = nil // Suppress keytool output
	cmd.Stderr = nil
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create JKS truststore: %w", err)
	}

	// Clean up intermediate file
	os.Remove(p12Path)

	return nil
}

// generateInvalidCertificates creates deliberately invalid certificates for negative testing
func (tc *TestCertificates) generateInvalidCertificates() error {
	// Create expired certificate (for testing expiration handling)
	expiredCertPath := filepath.Join(tc.tempDir, "expired.crt")
	expiredKeyPath := filepath.Join(tc.tempDir, "expired.key")
	
	// Use openssl to create an expired certificate
	cmd := exec.Command("openssl", "req", "-x509", "-newkey", "rsa:2048", "-keyout", expiredKeyPath,
		"-out", expiredCertPath, "-days", "-1", "-nodes", "-subj", "/CN=expired.test")
	
	if _, err := cmd.CombinedOutput(); err != nil {
		// If openssl isn't available, skip invalid certificate generation
		if tc.t != nil {
			tc.t.Logf("Warning: Could not generate invalid certificates (openssl not available): %v", err)
		}
		return nil
	}

	// Create certificate with wrong hostname (for hostname validation testing)
	wrongHostCertPath := filepath.Join(tc.tempDir, "wronghost.crt")
	wrongHostKeyPath := filepath.Join(tc.tempDir, "wronghost.key")
	
	cmd = exec.Command("openssl", "req", "-x509", "-newkey", "rsa:2048", "-keyout", wrongHostKeyPath,
		"-out", wrongHostCertPath, "-days", "1", "-nodes", "-subj", "/CN=wrong.hostname.test")
	
	if output, err := cmd.CombinedOutput(); err != nil {
		if tc.t != nil {
			tc.t.Logf("Warning: Could not generate wrong hostname certificate: %v\nOutput: %s", err, string(output))
		}
	}

	return nil
}

// GetInvalidCertificatePaths returns paths to invalid certificates for negative testing
func (tc *TestCertificates) GetInvalidCertificatePaths() (expiredCert, wrongHostCert string) {
	expiredCert = filepath.Join(tc.tempDir, "expired.crt")
	wrongHostCert = filepath.Join(tc.tempDir, "wronghost.crt")
	return
}