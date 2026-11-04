// Package projectcli implements the `issueops project` subcommands (bootstrap,
// docs, route-docs, append, commit-suggest, lint-diagnose) as an inbound CLI
// adapter over injected application services.
package projectcli

import "issueops/cmd/issueops/jsonout"

var printJSON = jsonout.Print
