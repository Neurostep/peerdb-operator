/*
Copyright 2026.

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

package v1alpha1

import (
	"fmt"
	"regexp"
	"strings"
)

// versionRegex matches semantic versions like "v0.36.7", "0.36.7", "v1.2.3-rc1"
var versionRegex = regexp.MustCompile(`v?(\d+)\.(\d+)(?:\.\d+)?`)

// MajorMinorFromVersion extracts "major.minor" from a version string like "v0.36.7".
// Returns e.g. "0.36".
func MajorMinorFromVersion(version string) (string, error) {
	matches := versionRegex.FindStringSubmatch(version)
	if matches == nil {
		return "", fmt.Errorf("cannot parse version %q", version)
	}
	return matches[1] + "." + matches[2], nil
}

// MajorMinorFromImage extracts "major.minor" from a container image tag.
// Supports formats like "ghcr.io/peerdb-io/flow-worker:stable-v0.36.7" or "custom:v1.2.3".
func MajorMinorFromImage(image string) (string, error) {
	// Extract the tag part after the last ":"
	parts := strings.SplitN(image, ":", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("image %q has no tag", image)
	}
	tag := parts[1]

	// If the tag starts with "sha256:" it's a digest, we can't check skew
	if strings.HasPrefix(tag, "sha256:") {
		return "", fmt.Errorf("image %q uses a digest, cannot determine version", image)
	}

	matches := versionRegex.FindStringSubmatch(tag)
	if matches == nil {
		return "", fmt.Errorf("cannot parse version from image tag %q", tag)
	}
	return matches[1] + "." + matches[2], nil
}
