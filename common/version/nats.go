package version

import version2 "github.com/hashicorp/go-version"

// NatsVersion is the mandatory minimum version of NATS that is supported by SHAR
var NatsVersion, _ = version2.NewVersion("v2.10.12")
