package gateway

import (
	"context"
)

// emit sends one event to the stream channel or returns ctx.Err() after emitting an error event if possible.
func emit(ctx context.Context, events chan<- StreamEvent, ev StreamEvent) error {
	select {
	case <-ctx.Done():
		err := ctx.Err()
		select {
		case events <- StreamEvent{Type: "error", Error: err}:
		default:
		}
		return err
	case events <- ev:
		return nil
	}
}
