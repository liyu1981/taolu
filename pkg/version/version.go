// Package version holds the taolu build version, set at compile time via
//
//	-ldflags "-X github.com/yli/taolu/pkg/version.Version=v1.2.3"
package version

// Version is the build version. It defaults to "dev" and is overridden by
// -ldflags at build time.
var Version = "dev"
