package history

// ContentUpdate adds a new message to the log.
type ContentUpdate struct {
	Message any
}

// ContentRefresh replaces the current log with its own log.
type ContentRefresh struct {
	Log []any
}
