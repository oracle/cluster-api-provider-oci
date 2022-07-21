/*
Copyright 2021 The Kubernetes Authors.

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

package repository

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/version"

	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/internal/scheme"
)

const (
	latestVersionTag = "latest"
)

// latestContractRelease returns the latest patch release for a repository for the current API contract, according to
// semantic version order of the release tag name.
func latestContractRelease(repo Repository, contract string) (string, error) {
	latest, err := latestRelease(repo)
	if err != nil {
		return latest, err
	}
	// Attempt to check if the latest release satisfies the API Contract
	// This is a best-effort attempt to find the latest release for an older API contract if it's not the latest release.
	// If an error occurs, we just return the latest release.
	file, err := repo.GetFile(latest, metadataFile)
	if err != nil {
		// if we can't get the metadata file from the release, we return latest.
		return latest, nil //nolint:nilerr
	}
	latestMetadata := &clusterctlv1.Metadata{}
	codecFactory := serializer.NewCodecFactory(scheme.Scheme)
	if err := runtime.DecodeInto(codecFactory.UniversalDecoder(), file, latestMetadata); err != nil {
		return latest, nil //nolint:nilerr
	}

	releaseSeries := latestMetadata.GetReleaseSeriesForContract(contract)
	if releaseSeries == nil {
		return latest, nil
	}

	sv, err := version.ParseSemantic(latest)
	if err != nil {
		return latest, nil //nolint:nilerr
	}

	// If the Major or Minor version of the latest release doesn't match the release series for the current contract,
	// return the latest patch release of the desired Major/Minor version.
	if sv.Major() != releaseSeries.Major || sv.Minor() != releaseSeries.Minor {
		return latestPatchRelease(repo, &releaseSeries.Major, &releaseSeries.Minor)
	}
	return latest, nil
}

// latestRelease returns the latest release for a repository, according to
// semantic version order of the release tag name.
func latestRelease(repo Repository) (string, error) {
	return latestPatchRelease(repo, nil, nil)
}

// latestPatchRelease returns the latest patch release for a given Major and Minor version.
func latestPatchRelease(repo Repository, major, minor *uint) (string, error) {
	versions, err := repo.GetVersions()
	if err != nil {
		return "", errors.Wrapf(err, "failed to get repository versions")
	}

	// Search for the latest release according to semantic version ordering.
	// Releases with tag name that are not in semver format are ignored.
	var latestTag string
	var latestPrereleaseTag string

	var latestReleaseVersion *version.Version
	var latestPrereleaseVersion *version.Version

	for _, v := range versions {
		sv, err := version.ParseSemantic(v)
		if err != nil {
			// discard releases with tags that are not a valid semantic versions (the user can point explicitly to such releases)
			continue
		}

		if (major != nil && sv.Major() != *major) || (minor != nil && sv.Minor() != *minor) {
			// skip versions that don't match the desired Major.Minor version.
			continue
		}

		// track prereleases separately
		if sv.PreRelease() != "" {
			if latestPrereleaseVersion == nil || latestPrereleaseVersion.LessThan(sv) {
				latestPrereleaseTag = v
				latestPrereleaseVersion = sv
			}
			continue
		}

		if latestReleaseVersion == nil || latestReleaseVersion.LessThan(sv) {
			latestTag = v
			latestReleaseVersion = sv
		}
	}

	// Fall back to returning latest prereleases if no release has been cut or bail if it's also empty
	if latestTag == "" {
		if latestPrereleaseTag == "" {
			return "", errors.New("failed to find releases tagged with a valid semantic version number")
		}

		return latestPrereleaseTag, nil
	}
	return latestTag, nil
}
