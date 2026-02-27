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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// set this way to allow for mocks to be used for testing
var instancePrincipalProviderFunc = auth.InstancePrincipalConfigurationProvider
var sessionTokenProviderFunc = common.ConfigurationProviderForSessionToken

const (
	UseInstancePrincipal = "useInstancePrincipal"
	UseSessionToken      = "useSessionToken"
	Tenancy              = "tenancy"
	User                 = "user"
	Passphrase           = "passphrase"
	Key                  = "key"
	SessionToken         = "sessionToken"
	SessionPrivateKey    = "sessionPrivateKey"
	Fingerprint          = "fingerprint"
	Region               = "region"
)

// AuthConfig holds the configuration required for communicating with the OCI
// API.
type AuthConfig struct {
	Region                string `yaml:"region"`
	TenancyID             string `yaml:"tenancy"`
	UserID                string `yaml:"user"`
	PrivateKey            string `yaml:"key"`
	SessionTokenFilePath  string `yaml:"sessionTokenFilePath"`
	SessionPrivateKeyPath string `yaml:"sessionPrivateKeyPath"`
	Fingerprint           string `yaml:"fingerprint"`
	Passphrase            string `yaml:"passphrase"`
	UseInstancePrincipals bool   `yaml:"useInstancePrincipals"`
	UseSessionToken       bool   `yaml:"useSessionToken"`
}

// FromDir will load a cloud provider configuration file from a given directory.
func FromDir(path string) (*AuthConfig, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if fileInfo.IsDir() {
		return getConfigFromDir(path)
	} else {
		return getConfigFromFile(path)
	}
}

func getConfigFromFile(path string) (authConfig *AuthConfig, err error) {
	var f *os.File
	f, err = os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = closeErr
		}
	}()

	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, errors.New("no auth config file given")
	}

	cfg := &AuthConfig{}
	err = yaml.NewDecoder(f).Decode(&cfg)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling auth config")
	}

	return cfg, nil
}

func getConfigFromDir(path string) (*AuthConfig, error) {
	cfg := &AuthConfig{}
	useInstancePrincipalString, err := ReadFile(path, UseInstancePrincipal)
	if err != nil {
		return nil, err
	}
	useInstancePrincipal, err := strconv.ParseBool(strings.TrimSpace(useInstancePrincipalString))
	if err != nil {
		return nil, err
	}
	if useInstancePrincipal {
		cfg.UseInstancePrincipals = useInstancePrincipal
		return cfg, nil
	}
	useSessionToken, err := readBoolFile(path, UseSessionToken, false)
	if err != nil {
		return nil, err
	}
	if useSessionToken {
		cfg.UseSessionToken = true

		region, err := ReadFile(path, Region)
		if err != nil {
			return nil, err
		}
		cfg.Region = region

		tenancy, err := ReadFile(path, Tenancy)
		if err != nil {
			return nil, err
		}
		cfg.TenancyID = tenancy

		fingerprint, err := ReadFile(path, Fingerprint)
		if err != nil {
			return nil, err
		}
		cfg.Fingerprint = fingerprint

		// Validate required files and keep file paths for refreshable session token auth.
		if _, err := ReadFile(path, SessionToken); err != nil {
			return nil, err
		}
		if _, err := ReadFile(path, SessionPrivateKey); err != nil {
			return nil, err
		}
		cfg.SessionTokenFilePath = filepath.Join(path, SessionToken)
		cfg.SessionPrivateKeyPath = filepath.Join(path, SessionPrivateKey)

		passphrase, err := ReadFile(path, Passphrase)
		if err == nil {
			cfg.Passphrase = passphrase
		}
		return cfg, nil
	}
	region, err := ReadFile(path, Region)
	if err != nil {
		return nil, err
	}
	cfg.Region = region

	tenancy, err := ReadFile(path, Tenancy)
	if err != nil {
		return nil, err
	}
	cfg.TenancyID = tenancy

	user, err := ReadFile(path, User)
	if err != nil {
		return nil, err
	}
	cfg.UserID = user

	fingerprint, err := ReadFile(path, Fingerprint)
	if err != nil {
		return nil, err
	}
	cfg.Fingerprint = fingerprint

	passphrase, err := ReadFile(path, Passphrase)
	if err != nil {
		return nil, err
	}
	cfg.Passphrase = passphrase

	key, err := ReadFile(path, Key)
	if err != nil {
		return nil, err
	}
	cfg.PrivateKey = key

	return cfg, nil
}

func NewConfigurationProvider(cfg *AuthConfig) (common.ConfigurationProvider, error) {
	if cfg == nil {
		return nil, errors.New("auth config must not be nil")
	}
	if cfg.UseInstancePrincipals {
		return instancePrincipalProviderFunc()
	}
	if cfg.UseSessionToken {
		return NewConfigurationProviderWithSessionToken(cfg)
	}
	return NewConfigurationProviderWithUserPrincipal(cfg)
}

// nolint:nilnil
func NewConfigurationProviderWithUserPrincipal(cfg *AuthConfig) (common.ConfigurationProvider, error) {
	var conf common.ConfigurationProvider
	if cfg == nil {
		return nil, errors.New("cfg cannot be nil")
	}
	conf = common.NewRawConfigurationProvider(
		cfg.TenancyID,
		cfg.UserID,
		cfg.Region,
		cfg.Fingerprint,
		cfg.PrivateKey,
		common.String(cfg.Passphrase))
	return conf, nil
}

func NewConfigurationProviderWithSessionToken(cfg *AuthConfig) (common.ConfigurationProvider, error) {
	if cfg == nil {
		return nil, errors.New("cfg cannot be nil")
	}
	if cfg.TenancyID == "" || cfg.Region == "" || cfg.Fingerprint == "" || cfg.SessionPrivateKeyPath == "" || cfg.SessionTokenFilePath == "" {
		return nil, errors.New("session token auth requires tenancy, region, fingerprint, sessionPrivateKeyPath and sessionTokenFilePath")
	}
	configPath, err := writeSessionTokenConfigFile(cfg)
	if err != nil {
		return nil, err
	}
	return sessionTokenProviderFunc(configPath, strings.TrimSpace(cfg.Passphrase))
}

func writeSessionTokenConfigFile(cfg *AuthConfig) (string, error) {
	tmp, err := os.CreateTemp("", "oci-session-token-auth-*")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tmp.Close()
	}()

	cfgLines := []string{
		"[DEFAULT]",
		fmt.Sprintf("tenancy=%s", strings.TrimSpace(cfg.TenancyID)),
		fmt.Sprintf("region=%s", strings.TrimSpace(cfg.Region)),
		fmt.Sprintf("fingerprint=%s", strings.TrimSpace(cfg.Fingerprint)),
		fmt.Sprintf("key_file=%s", strings.TrimSpace(cfg.SessionPrivateKeyPath)),
		fmt.Sprintf("security_token_file=%s", strings.TrimSpace(cfg.SessionTokenFilePath)),
	}
	if strings.TrimSpace(cfg.Passphrase) != "" {
		cfgLines = append(cfgLines, fmt.Sprintf("pass_phrase=%s", strings.TrimSpace(cfg.Passphrase)))
	}
	if _, err := tmp.WriteString(strings.Join(cfgLines, "\n") + "\n"); err != nil {
		return "", err
	}
	return tmp.Name(), nil
}

func ReadFile(path string, key string) (string, error) {
	filePath := filepath.Join(path, key)
	b, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return "", err
	}
	return string(b), err
}

func readBoolFile(path string, key string, defaultValue bool) (bool, error) {
	rawValue, err := ReadFile(path, key)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultValue, nil
		}
		return false, err
	}
	return strconv.ParseBool(strings.TrimSpace(rawValue))
}
