/*
Copyright (c) 2026 Oracle and/or its affiliates.

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

package config_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

type e2eConfig struct {
	Providers []struct {
		Name     string `yaml:"name"`
		Type     string `yaml:"type"`
		Versions []struct {
			Name  string            `yaml:"name"`
			Files []providerFileRef `yaml:"files"`
		} `yaml:"versions"`
	} `yaml:"providers"`
}

type providerFileRef struct {
	SourcePath string `yaml:"sourcePath"`
}

type providerMetadata struct {
	ReleaseSeries []struct {
		Major    int    `yaml:"major"`
		Minor    int    `yaml:"minor"`
		Contract string `yaml:"contract"`
	} `yaml:"releaseSeries"`
}

func TestCAPIProviderVersionsHaveReleaseSeriesMetadata(t *testing.T) {
	configPath := "e2e_conf.yaml"
	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read E2E configuration: %v", err)
	}

	config := &e2eConfig{}
	if err := yaml.Unmarshal(contents, config); err != nil {
		t.Fatalf("parse E2E configuration: %v", err)
	}

	capiProviderTypes := map[string]struct{}{
		"CoreProvider":         {},
		"BootstrapProvider":    {},
		"ControlPlaneProvider": {},
	}
	for _, provider := range config.Providers {
		if _, ok := capiProviderTypes[provider.Type]; !ok {
			continue
		}
		for _, version := range provider.Versions {
			major, minor, err := parseMajorMinor(version.Name)
			if err != nil {
				t.Errorf("provider %s %s: %v", provider.Name, provider.Type, err)
				continue
			}

			metadataPaths := metadataFiles(version.Files)
			if len(metadataPaths) == 0 {
				t.Errorf("provider %s %s version %s has no metadata file", provider.Name, provider.Type, version.Name)
				continue
			}
			for _, path := range metadataPaths {
				metadataPath := filepath.Clean(filepath.Join(filepath.Dir(configPath), path))
				metadataContents, err := os.ReadFile(metadataPath)
				if err != nil {
					t.Errorf("provider %s %s version %s: read %s: %v", provider.Name, provider.Type, version.Name, metadataPath, err)
					continue
				}
				metadata := &providerMetadata{}
				if err := yaml.Unmarshal(metadataContents, metadata); err != nil {
					t.Errorf("provider %s %s version %s: parse %s: %v", provider.Name, provider.Type, version.Name, metadataPath, err)
					continue
				}
				expectedContract := filepath.Base(filepath.Dir(metadataPath))
				if !hasReleaseSeries(metadata, major, minor, expectedContract) {
					t.Errorf("provider %s %s version %s has no matching %s release series in %s", provider.Name, provider.Type, version.Name, expectedContract, metadataPath)
				}
			}
		}
	}
}

func metadataFiles(files []providerFileRef) []string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		if filepath.Base(file.SourcePath) == "metadata.yaml" {
			paths = append(paths, file.SourcePath)
		}
	}
	return paths
}

func parseMajorMinor(version string) (int, int, error) {
	if !strings.HasPrefix(version, "v") {
		return 0, 0, fmt.Errorf("version %q must start with v", version)
	}
	parts := strings.Split(strings.TrimPrefix(version, "v"), ".")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("version %q must contain major and minor components", version)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parse major version %q: %w", parts[0], err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parse minor version %q: %w", parts[1], err)
	}
	return major, minor, nil
}

func hasReleaseSeries(metadata *providerMetadata, major, minor int, contract string) bool {
	for _, releaseSeries := range metadata.ReleaseSeries {
		if releaseSeries.Major == major && releaseSeries.Minor == minor && releaseSeries.Contract == contract {
			return true
		}
	}
	return false
}
