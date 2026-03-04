package resources

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// ComputeConfigHash computes a deterministic SHA-256 hash from ConfigMap data
// and Secret resource versions. The hash changes when config data or secrets change,
// triggering pod rollouts via the config-hash annotation.
func ComputeConfigHash(configData map[string]string, secretResourceVersions map[string]string) string {
	h := sha256.New()

	// Write sorted config data
	keys := make([]string, 0, len(configData))
	for k := range configData {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(h, "config:%s=%s\n", k, configData[k])
	}

	// Write sorted secret resource versions
	secretKeys := make([]string, 0, len(secretResourceVersions))
	for k := range secretResourceVersions {
		secretKeys = append(secretKeys, k)
	}
	sort.Strings(secretKeys)
	for _, k := range secretKeys {
		fmt.Fprintf(h, "secret:%s=%s\n", k, secretResourceVersions[k])
	}

	return hex.EncodeToString(h.Sum(nil))[:16]
}

// SanitizeVersion converts a version string to a format suitable for Kubernetes resource names.
// e.g., "v0.36.7" -> "v0-36-7"
func SanitizeVersion(version string) string {
	return strings.ReplaceAll(version, ".", "-")
}
