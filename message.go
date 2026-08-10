package slack

import (
	contractsnotification "github.com/goravel/framework/contracts/notification"
)

type Notification interface {
	contractsnotification.Notification
	// ToSlack returns the Message to post via the Slack Web API.
	ToSlack(notifiable contractsnotification.Notifiable) Message
}

type Message struct {
	// Text is the fallback/plain-text message body.
	Text string
	// Attachments are legacy secondary-message attachments — still
	// supported by chat.postMessage, simpler than Block Kit for basic
	// "title + fields + color" use cases.
	Attachments []Attachment
	// ThreadTS, if set, posts this message as a reply in the given
	// thread (Slack's "ts" of the parent message).
	ThreadTS string
}

// Attachment is a single legacy Slack message attachment.
type Attachment struct {
	// Title is the bold attachment title.
	Title string
	// Text is the attachment body text.
	Text string
	// Color is "good", "warning", "danger", or a hex string like "#36a64f".
	Color string
	// Fields are key-value pairs displayed in a table inside the attachment.
	Fields []Field
	// Footer is small text shown at the bottom of the attachment.
	Footer string
	// Timestamp is a Unix timestamp shown in the attachment footer.
	Timestamp int64
}

// Field is a single key-value pair inside an Attachment.
type Field struct {
	// Title is the field label.
	Title string
	// Value is the field content (supports Slack mrkdwn).
	Value string
	// Short controls whether the field appears side-by-side with other short fields.
	Short bool
}
