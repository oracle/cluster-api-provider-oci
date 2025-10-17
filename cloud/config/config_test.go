/*
Copyright (c) 2021, 2022 Oracle and/or its affiliates.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"crypto/rsa"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
)

// mockConfigurationProvider implements common.ConfigurationProvider for testing
type mockInstancePrincipalConfigurationProvider struct{}

func (m *mockInstancePrincipalConfigurationProvider) TenancyOCID() (string, error) {
	return "mock-tenancy", nil
}

func (m *mockInstancePrincipalConfigurationProvider) UserOCID() (string, error) {
	return "mock-user", nil
}

func (m *mockInstancePrincipalConfigurationProvider) KeyID() (string, error) {
	return "mock-key-id", nil
}

func (m *mockInstancePrincipalConfigurationProvider) KeyFingerprint() (string, error) {
	return "mock-fingerprint", nil
}

func (m *mockInstancePrincipalConfigurationProvider) Key() (string, error) {
	return "mock-key", nil
}

func (m *mockInstancePrincipalConfigurationProvider) PrivateRSAKey() (*rsa.PrivateKey, error) {
	return nil, nil // Mock implementation
}

func (m *mockInstancePrincipalConfigurationProvider) Region() (string, error) {
	return "us-phoenix-1", nil
}

func (m *mockInstancePrincipalConfigurationProvider) AuthType() (common.AuthConfig, error) {
	return common.AuthConfig{AuthType: "instance_principal"}, nil
}

func TestFromDir(t *testing.T) {
	type args struct {
		path string
	}

	// Setup: valid config file
	validContent := []byte(`
region: us-phoenix-1
tenancy: ocid1.tenancy.oc1..aaaaaaa
user: ocid1.user.oc1..bbbbbbb
key: some-private-key
fingerprint: 11:22:33:44
passphrase: secret-pass
useInstancePrincipals: true
`)
	tmpFile, err := os.CreateTemp("", "auth-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(validContent); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// Setup: directory path (empty, so expect error from getConfigFromDir)
	tmpDir, err := os.MkdirTemp("", "auth-dir-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name    string
		args    args
		want    *AuthConfig
		wantErr bool
	}{
		{
			name:    "non-existent path",
			args:    args{path: "no-such-path"},
			want:    nil,
			wantErr: true,
		},
		{
			name: "valid file path",
			args: args{path: tmpFile.Name()},
			want: &AuthConfig{
				Region:                "us-phoenix-1",
				TenancyID:             "ocid1.tenancy.oc1..aaaaaaa",
				UserID:                "ocid1.user.oc1..bbbbbbb",
				PrivateKey:            "some-private-key",
				Fingerprint:           "11:22:33:44",
				Passphrase:            "secret-pass",
				UseInstancePrincipals: true,
			},
			wantErr: false,
		},
		{
			name:    "directory path",
			args:    args{path: tmpDir},
			want:    nil,  // since empty dir will make getConfigFromDir fail
			wantErr: true, // adjust depending on getConfigFromDir impl
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromDir(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FromDir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getConfigFromFile(t *testing.T) {
	type args struct {
		path string
	}

	// Create a temporary valid config file
	validContent := []byte(`
region: us-phoenix-1
tenancy: ocid1.tenancy.oc1..aaaaaaa
user: ocid1.user.oc1..bbbbbbb
key: some-private-key
fingerprint: 11:22:33:44
passphrase: secret-pass
useInstancePrincipals: true
`)
	tmpValid, err := os.CreateTemp("", "valid-auth-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpValid.Name())
	if _, err := tmpValid.Write(validContent); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpValid.Close()

	tests := []struct {
		name           string
		args           args
		wantAuthConfig *AuthConfig
		wantErr        bool
	}{
		{
			name:    "file does not exist",
			args:    args{path: "non-existent-file.yaml"},
			wantErr: true,
		},
		{
			name: "empty file",
			args: args{path: func() string {
				f, _ := os.CreateTemp("", "empty-auth-*.yaml")
				defer f.Close()
				return f.Name()
			}()},
			wantErr: true,
		},
		{
			name: "valid yaml",
			args: args{path: tmpValid.Name()},
			wantAuthConfig: &AuthConfig{
				Region:                "us-phoenix-1",
				TenancyID:             "ocid1.tenancy.oc1..aaaaaaa",
				UserID:                "ocid1.user.oc1..bbbbbbb",
				PrivateKey:            "some-private-key",
				Fingerprint:           "11:22:33:44",
				Passphrase:            "secret-pass",
				UseInstancePrincipals: true,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAuthConfig, err := getConfigFromFile(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("getConfigFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotAuthConfig, tt.wantAuthConfig) {
				t.Errorf("getConfigFromFile() = %v, want %v", gotAuthConfig, tt.wantAuthConfig)
			}
		})
	}
}

func Test_getConfigFromDir(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name    string
		args    args
		want    *AuthConfig
		wantErr bool
	}{
		{
			name:    "missing useInstancePrincipals file",
			args:    args{path: t.TempDir()},
			want:    nil,
			wantErr: true,
		},
		{
			name: "useInstancePrincipals true",
			args: args{path: func() string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, UseInstancePrincipal), []byte("true"), 0644)
				return dir
			}()},
			want: &AuthConfig{
				UseInstancePrincipals: true,
			},
			wantErr: false,
		},
		{
			name: "useInstancePrincipals false but missing other files",
			args: args{path: func() string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, UseInstancePrincipal), []byte("false"), 0644)
				return dir
			}()},
			want:    nil,
			wantErr: true,
		},
		{
			name: "all files present with valid content",
			args: args{path: func() string {
				dir := t.TempDir()
				os.WriteFile(filepath.Join(dir, UseInstancePrincipal), []byte("false"), 0644)
				os.WriteFile(filepath.Join(dir, Region), []byte("us-phoenix-1"), 0644)
				os.WriteFile(filepath.Join(dir, Tenancy), []byte("ocid1.tenancy.oc1..aaaaaaa"), 0644)
				os.WriteFile(filepath.Join(dir, User), []byte("ocid1.user.oc1..bbbbbbb"), 0644)
				os.WriteFile(filepath.Join(dir, Fingerprint), []byte("11:22:33:44"), 0644)
				os.WriteFile(filepath.Join(dir, Passphrase), []byte("secret-pass"), 0644)
				os.WriteFile(filepath.Join(dir, Key), []byte("some-private-key"), 0644)
				return dir
			}()},
			want: &AuthConfig{
				Region:                "us-phoenix-1",
				TenancyID:             "ocid1.tenancy.oc1..aaaaaaa",
				UserID:                "ocid1.user.oc1..bbbbbbb",
				PrivateKey:            "some-private-key",
				Fingerprint:           "11:22:33:44",
				Passphrase:            "secret-pass",
				UseInstancePrincipals: false,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getConfigFromDir(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("getConfigFromDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getConfigFromDir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewConfigurationProvider(t *testing.T) {
	type args struct {
		cfg *AuthConfig
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		setup   func()
		cleanup func()
	}{
		{
			name:    "nil config",
			args:    args{cfg: nil},
			wantErr: true,
		},
		{
			name: "use instance principals",
			args: args{cfg: &AuthConfig{
				UseInstancePrincipals: true,
			}},
			wantErr: false,
			setup: func() {
				instancePrincipalProviderFunc = func() (common.ConfigurationProvider, error) {
					return &mockInstancePrincipalConfigurationProvider{}, nil
				}
			},
			cleanup: func() {
				instancePrincipalProviderFunc = auth.InstancePrincipalConfigurationProvider
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			if tt.cleanup != nil {
				defer tt.cleanup()
			}
			got, err := NewConfigurationProvider(tt.args.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfigurationProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Errorf("NewConfigurationProvider() returned nil, want non-nil")
			}
		})
	}
}

func TestNewConfigurationProviderWithUserPrincipal(t *testing.T) {
	type args struct {
		cfg *AuthConfig
	}
	tests := []struct {
		name    string
		args    args
		want    common.ConfigurationProvider
		wantErr bool
	}{
		{
			name:    "nil config",
			args:    args{cfg: nil},
			want:    nil,
			wantErr: true,
		},
		{
			name: "use user principals",
			args: args{cfg: &AuthConfig{
				Region:      "us-phoenix-1",
				TenancyID:   "ocid1.tenancy.oc1..aaaaaaa",
				UserID:      "ocid1.user.oc1..bbbbbbb",
				PrivateKey:  "some-private-key",
				Fingerprint: "11:22:33:44",
				Passphrase:  "secret-pass",
			}},
			want: common.NewRawConfigurationProvider(
				"ocid1.tenancy.oc1..aaaaaaa",
				"ocid1.user.oc1..bbbbbbb",
				"us-phoenix-1",
				"11:22:33:44",
				"some-private-key",
				common.String("secret-pass")),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewConfigurationProviderWithUserPrincipal(tt.args.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfigurationProviderWithUserPrincipal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewConfigurationProviderWithUserPrincipal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadFile(t *testing.T) {
	type args struct {
		path string
		key  string
	}

	// Setup: temporary directory and files
	tmpDir := t.TempDir()

	// File with content
	contentFile := "test-key"
	contentValue := "hello world"
	err := os.WriteFile(filepath.Join(tmpDir, contentFile), []byte(contentValue), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Empty file
	emptyFile := "empty-key"
	err = os.WriteFile(filepath.Join(tmpDir, emptyFile), []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to write empty file: %v", err)
	}

	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "file exists with content",
			args: args{
				path: tmpDir,
				key:  contentFile,
			},
			want:    contentValue,
			wantErr: false,
		},
		{
			name: "file does not exist",
			args: args{
				path: tmpDir,
				key:  "non-existent-key",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "empty file",
			args: args{
				path: tmpDir,
				key:  emptyFile,
			},
			want:    "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadFile(tt.args.path, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ReadFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
