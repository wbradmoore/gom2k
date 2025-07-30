package unit

import (
	"os"
	"path/filepath"
	"testing"
	
	"gom2k/pkg/validation"
)

func TestValidateSSLFilePath(t *testing.T) {
	// Create a temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.pem")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	
	tests := []struct {
		name        string
		path        string
		allowedDirs []string
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty path",
			path:        "",
			allowedDirs: nil,
			wantErr:     true,
			errContains: "cannot be empty",
		},
		{
			name:        "path traversal attempt",
			path:        "../../../etc/passwd",
			allowedDirs: nil,
			wantErr:     true,
			errContains: "path traversal",
		},
		{
			name:        "valid file no restrictions",
			path:        testFile,
			allowedDirs: nil,
			wantErr:     false,
		},
		{
			name:        "valid file in allowed dir",
			path:        testFile,
			allowedDirs: []string{tempDir},
			wantErr:     false,
		},
		{
			name:        "valid file not in allowed dir",
			path:        testFile,
			allowedDirs: []string{"/etc/ssl"},
			wantErr:     true,
			errContains: "not in allowed directories",
		},
		{
			name:        "non-existent file",
			path:        "/does/not/exist/file.pem",
			allowedDirs: nil,
			wantErr:     true,
			errContains: "does not exist",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateSSLFilePath(tt.path, tt.allowedDirs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSSLFilePath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errContains != "" && !contains(err.Error(), tt.errContains) {
				t.Errorf("ValidateSSLFilePath() error = %v, want error containing %s", err, tt.errContains)
			}
		})
	}
}

func TestValidateConfigPath(t *testing.T) {
	// Create a temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	
	tests := []struct {
		name        string
		path        string
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty path",
			path:        "",
			wantErr:     true,
			errContains: "cannot be empty",
		},
		{
			name:    "valid config path",
			path:    testFile,
			wantErr: false,
		},
		{
			name:        "non-existent file",
			path:        "/does/not/exist.yaml",
			wantErr:     true,
			errContains: "does not exist",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateConfigPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfigPath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errContains != "" && !contains(err.Error(), tt.errContains) {
				t.Errorf("ValidateConfigPath() error = %v, want error containing %s", err, tt.errContains)
			}
		})
	}
}

