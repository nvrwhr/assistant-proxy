package main

// Memory defines the interface for storing chat messages.
type Memory interface {
	SaveMessage(threadID string, msg string) error
	GetMessages(threadID string) ([]string, error)
}
