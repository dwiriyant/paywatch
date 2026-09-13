package admin

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

// ponytail: in-memory pending sessions so verify → create within 1m skips a second GoBiz login.
// Ceiling: single-process only; multi-replica needs shared store.
const pendingSessionTTL = time.Minute

type pendingEntry struct {
	sess    AuthSession
	expires time.Time
}

type pendingSessions struct {
	mu   sync.Mutex
	byKey map[string]pendingEntry
}

func newPendingSessions() *pendingSessions {
	return &pendingSessions{byKey: map[string]pendingEntry{}}
}

func pendingKey(email, password string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email)) + "\n" + password))
	return hex.EncodeToString(sum[:])
}

func (p *pendingSessions) Put(email, password string, sess AuthSession) {
	if p == nil || email == "" || password == "" || sess.AccessToken == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gcLocked(time.Now())
	p.byKey[pendingKey(email, password)] = pendingEntry{
		sess:    sess,
		expires: time.Now().Add(pendingSessionTTL),
	}
}

func (p *pendingSessions) Get(email, password string) (AuthSession, bool) {
	if p == nil {
		return AuthSession{}, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	p.gcLocked(now)
	k := pendingKey(email, password)
	e, ok := p.byKey[k]
	if !ok || now.After(e.expires) {
		delete(p.byKey, k)
		return AuthSession{}, false
	}
	return e.sess, true
}

func (p *pendingSessions) Clear(email, password string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	delete(p.byKey, pendingKey(email, password))
	p.mu.Unlock()
}

func (p *pendingSessions) gcLocked(now time.Time) {
	for k, e := range p.byKey {
		if now.After(e.expires) {
			delete(p.byKey, k)
		}
	}
}
