// Event definition layer
// The contract between producers and consumers.
// Both sides import this package to keep serialization and deserialization consistent.
package events

const UserRegisteredVersion = "v1"

// UserRegistered event struct
type UserRegistered struct {
	Version   string `json:"version"`
	EventType string `json:"event_type"`
	UserID    int64  `json:"user_id"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	Timestamp int64  `json:"timestamp"`
}
