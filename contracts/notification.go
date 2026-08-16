package contracts

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
	Text string `json:"text"`
	// Attachments are legacy secondary-message attachments — still
	// supported by chat.postMessage, simpler than Block Kit for basic
	// "title + fields + color" use cases.
	Attachments []Attachment `json:"attachments,omitempty"`

	ThreadTS string `json:"thread_ts,omitempty"`
}

// Attachment is a single legacy Slack message attachment.
type Attachment struct {
	// Title is the bold attachment title.
	Title string `json:"title,omitempty"`
	// Text is the attachment body text.
	Text string `json:"text,omitempty"`
	// Color is "good", "warning", "danger", or a hex string like "#36a64f".
	Color string `json:"color,omitempty"`
	// Fields are key-value pairs displayed in a table inside the attachment.
	Fields []Field `json:"fields,omitempty"`
	// Footer is small text shown at the bottom of the attachment.
	Footer    string `json:"footer,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// Field is a single key-value pair inside an Attachment.
type Field struct {
	// Title is the field label.
	Title string `json:"title"`
	// Value is the field content (supports Slack mrkdwn).
	Value string `json:"value"`
	// Short controls whether the field appears side-by-side with other short fields.
	Short bool `json:"short"`
}
