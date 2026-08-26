package application

import (
	"context"
	"sync"

	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
)

type reviewCall struct {
	done chan struct{}
	risk domain.RiskAssessment
	err  error
}

type reviewGroup struct {
	mu   sync.Mutex
	call *reviewCall
}

func (g *reviewGroup) do(ctx context.Context, fn func() (domain.RiskAssessment, error)) (domain.RiskAssessment, error) {
	g.mu.Lock()
	if g.call != nil {
		call := g.call
		g.mu.Unlock()
		select {
		case <-call.done:
			return call.risk, call.err
		case <-ctx.Done():
			return domain.RiskAssessment{}, ctx.Err()
		}
	}
	call := &reviewCall{done: make(chan struct{})}
	g.call = call
	g.mu.Unlock()

	call.risk, call.err = fn()
	close(call.done)

	g.mu.Lock()
	if g.call == call {
		g.call = nil
	}
	g.mu.Unlock()
	return call.risk, call.err
}
