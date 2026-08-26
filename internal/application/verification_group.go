package application

import (
	"context"
	"sync"

	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
)

type verificationCall struct {
	done       chan struct{}
	credential domain.Credential
	caseRecord domain.Case
	err        error
}

type verificationGroup struct {
	mu    sync.Mutex
	calls map[string]*verificationCall
}

func (g *verificationGroup) do(ctx context.Context, id string, verify func(context.Context, string) (domain.Credential, domain.Case, error)) (domain.Credential, domain.Case, error) {
	g.mu.Lock()
	if call := g.calls[id]; call != nil {
		g.mu.Unlock()
		select {
		case <-call.done:
			return call.credential, call.caseRecord, call.err
		case <-ctx.Done():
			return domain.Credential{}, domain.Case{}, ctx.Err()
		}
	}
	if g.calls == nil {
		g.calls = make(map[string]*verificationCall)
	}
	call := &verificationCall{done: make(chan struct{})}
	g.calls[id] = call
	g.mu.Unlock()

	call.credential, call.caseRecord, call.err = verify(ctx, id)

	g.mu.Lock()
	delete(g.calls, id)
	close(call.done)
	g.mu.Unlock()
	return call.credential, call.caseRecord, call.err
}
