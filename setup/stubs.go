package main

import "strings"

type Stubs struct{}

// Config mirrors goravel/framework's hash/setup/stubs.go structure
// exactly (github.com/goravel/framework/blob/master/hash/setup/stubs.go#L12-L18) —
// per hwbrzzl's review comment. A string template with placeholders
// substituted at generation time, not a static file, because the
// generated config needs to import the *consuming app's* actual
// facades package path, which varies per app and can't be known ahead
// of time the way a fixed file could be.
func (s Stubs) Config(pkg, facadesImport, facadesPackage string) string {
	content := `package DummyPackage

import (
	"DummyFacadesImport"
)

func init() {
	config := DummyFacadesPackage.Config()

	config.Add("slack", map[string]any{
		// token is the Slack bot token (xoxb-...). Create one at
		// https://api.slack.com/apps — needs the chat:write scope, and
		// the bot must be invited to any channel it should post in.
		"token": config.Env("SLACK_BOT_TOKEN", ""),
	})
}
`
	content = strings.ReplaceAll(content, "DummyPackage", pkg)
	content = strings.ReplaceAll(content, "DummyFacadesImport", facadesImport)
	content = strings.ReplaceAll(content, "DummyFacadesPackage", facadesPackage)
	return content
}
