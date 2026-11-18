// Package version exposes the application release version.
package version

import _ "embed"

// Current is the SemVer release number, shared by the CLI and build artifacts.
//
//go:embed VERSION
var Current string
