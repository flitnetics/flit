package types

// Notifier is the interface that all notification services have in common
type Notifier interface {
	StartNotification()
	SendNotification()
	GetNames() []string
	Close()
}
