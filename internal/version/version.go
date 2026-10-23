// Package version reports the build version.
package version

// Version is the build version; a -dev suffix marks a local build. Release
// builds set it with -ldflags -X.
var Version = "0.1.0-dev"
