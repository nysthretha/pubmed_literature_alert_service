package pubmed

import (
	"context"
	"time"
)

type limiter struct {
	tokens chan struct{}
}

func newLimiter(interval time.Duration) *limiter {
	l := &limiter{tokens: make(chan struct{}, 1)}
	l.tokens <- struct{}{} // prime so the first call doesn't wait
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for range t.C {
			select {
			case l.tokens <- struct{}{}:
			default:
			}
		}
	}()
	return l
}

func (l *limiter) Wait(ctx context.Context) error {
	select {
	case <-l.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
