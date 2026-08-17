package main

import "strings"

type Stubs struct{}

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
