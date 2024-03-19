package version

import (
	version2 "github.com/hashicorp/go-version"
)

// Version is the application version overwritten during build.
// During development this should be set to the minimum compatible version.
//
//goland:noinspection GoUnnecessarilyExportedIdentifiers
var Version = "v1.1.670"

// NatsVersion is the mandatory minimum version of NATS that is supported by SHAR
var NatsVersion, _ = version2.NewVersion("v2.10.12")
