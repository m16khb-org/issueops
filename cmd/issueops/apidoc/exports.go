// Package apidoc implements the `issueops api-doc` command: static API documentation checks, the review prompt
// and result handling, and the combined check, with text and JSON output.
package apidoc

import app "issueops/internal/application/apidoc"

// Command runs the `issueops api-doc` subcommands (check, review, static-check) against an application
// Service. ResolveTarget maps the repo flag to the target repository root.
type Command struct {
	Service       app.Service
	ResolveTarget func(string) string
}

// Run dispatches the api-doc subcommand in args. With no arguments it prints usage and fails.
func (c Command) Run(args []string) error { return c.runAPIDoc(args) }
