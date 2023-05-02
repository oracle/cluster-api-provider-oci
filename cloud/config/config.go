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
	"os"
	"strconv"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

const (
	UseInstancePrincipal = "useInstancePrincipal"
	Tenancy              = "tenancy"
	User                 = "user"
	Passphrase           = "passphrase"
	Key                  = "key"
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
	Fingerprint           string `yaml:"fingerprint"`
	Passphrase            string `yaml:"passphrase"`
	UseInstancePrincipals bool   `yaml:"useInstancePrincipals"`
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
	f, err = os.Open(path)
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
	useInstancePrincipal, err := strconv.ParseBool(useInstancePrincipalString)
	if err != nil {
		return nil, err
	}
	if useInstancePrincipal {
		cfg.UseInstancePrincipals = useInstancePrincipal
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
	if cfg.UseInstancePrincipals {
		return auth.InstancePrincipalConfigurationProvider()
	} else {
		return NewConfigurationProviderWithUserPrincipal(cfg)
	}
}

func NewConfigurationProviderWithUserPrincipal(cfg *AuthConfig) (common.ConfigurationProvider, error) {
	var conf common.ConfigurationProvider
	if cfg != nil {
		conf = common.NewRawConfigurationProvider(
			cfg.TenancyID,
			cfg.UserID,
			cfg.Region,
			cfg.Fingerprint,
			cfg.PrivateKey,
			common.String(cfg.Passphrase))
		return conf, nil
	}
	return nil, nil
}

func ReadFile(path string, key string) (string, error) {
	b, err := os.ReadFile(path + "/" + key)
	if err != nil {
		return "", err
	}
	return string(b), err
}
