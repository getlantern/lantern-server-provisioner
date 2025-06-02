package common

import "context"

type EventType int

const (
	EventTypeOAuthStarted EventType = iota
	EventTypeOAuthCompleted
	EventTypeOAuthCancelled
	EventTypeOAuthError
	EventTypeValidationStarted
	EventTypeValidationCompleted
	EventTypeValidationError
	EventTypeProvisioningStarted
	EventTypeProvisioningCompleted
	EventTypeProvisioningError
)

type Event struct {
	Type    EventType
	Error   error  // Error if the event is an error event.
	Message string // Additional information about the event, if applicable.
}

// Session represents an ongoing provisioning session.
type Session struct {
	Events chan Event
	Cancel context.CancelFunc
}
