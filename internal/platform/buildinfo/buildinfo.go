// Package buildinfo exposes non-sensitive build metadata for readiness and
// private-deployment diagnostics. Values are overridable at build time or by
// environment; defaults are intentionally safe for local development.
package buildinfo

import "os"

type Info struct {
	Version    string `json:"version"`
	Commit     string `json:"commit"`
	BuiltAt    string `json:"built_at"`
	APIVersion string `json:"api_version"`
}

func Current() Info {
	return Info{
		Version:    value("GP_VERSION", "dev"),
		Commit:     value("GP_COMMIT", "unknown"),
		BuiltAt:    value("GP_BUILT_AT", "unknown"),
		APIVersion: value("GP_API_VERSION", "0.1.0"),
	}
}

func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
