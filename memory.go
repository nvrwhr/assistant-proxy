package main

// Memory defines the interface for storing chat messages.
type Memory interface {
	SaveMessage(threadID string, msg Message) error
	GetMessages(threadID string) ([]Message, error)
}
