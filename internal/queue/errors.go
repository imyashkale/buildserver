package queue

import "errors"

// ErrQueueClosed is returned when trying to enqueue to a closed queue
var ErrQueueClosed = errors.New("queue is closed")
