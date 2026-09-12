package tenant

import "sync"

// AuthGate tracks tenants that must not hit the provider again after a fatal auth error.
type AuthGate struct {
	mu   sync.Mutex
	byApp map[string]string // app_id -> reason
}

func NewAuthGate() *AuthGate {
	return &AuthGate{byApp: map[string]string{}}
}

func (g *AuthGate) Block(appID, reason string) {
	if g == nil || appID == "" {
		return
	}
	g.mu.Lock()
	g.byApp[appID] = reason
	g.mu.Unlock()
}

func (g *AuthGate) Unblock(appID string) {
	if g == nil || appID == "" {
		return
	}
	g.mu.Lock()
	delete(g.byApp, appID)
	g.mu.Unlock()
}

func (g *AuthGate) Blocked(appID string) (reason string, ok bool) {
	if g == nil {
		return "", false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	reason, ok = g.byApp[appID]
	return reason, ok
}
