package proxy

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type circuitState int

const (
	stateClosed circuitState = iota
	stateOpen
	stateHalfOpen
)

type upstream struct {
	url       string
	state     circuitState
	failures  int
	lastFail  time.Time
	threshold int
	resetTTL  time.Duration
}

func (u *upstream) isAvailable() bool {
	switch u.state {
	case stateClosed:
		return true
	case stateHalfOpen:
		return true
	case stateOpen:
		if time.Since(u.lastFail) > u.resetTTL {
			u.state = stateHalfOpen
			return true
		}
		return false
	}
	return false
}

type Manager struct {
	mu        sync.Mutex
	upstreams []upstream
	index     int
}

func NewManager(urls []string, failureThreshold int, resetTimeout time.Duration) *Manager {
	if failureThreshold <= 0 {
		failureThreshold = 5
	}
	if resetTimeout <= 0 {
		resetTimeout = 30 * time.Second
	}

	upstreams := make([]upstream, len(urls))
	for i, u := range urls {
		upstreams[i] = upstream{
			url:       u,
			threshold: failureThreshold,
			resetTTL:  resetTimeout,
		}
	}
	return &Manager{upstreams: upstreams}
}

func (m *Manager) Pick() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.upstreams) == 0 {
		return ""
	}

	for range m.upstreams {
		m.index = (m.index + 1) % len(m.upstreams)
		u := &m.upstreams[m.index]
		if u.isAvailable() {
			return u.url
		}
	}

	log.Warn().Msg("no available upstream proxy")
	return ""
}

func (m *Manager) RecordSuccess(url string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.upstreams {
		if m.upstreams[i].url == url {
			m.upstreams[i].state = stateClosed
			m.upstreams[i].failures = 0
			break
		}
	}
}

func (m *Manager) RecordFailure(url string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.upstreams {
		if m.upstreams[i].url != url {
			continue
		}
		u := &m.upstreams[i]
		u.failures++
		u.lastFail = time.Now()

		if u.state == stateHalfOpen {
			u.state = stateOpen
			log.Warn().Str("proxy", url).Msg("circuit opened (half-open probe failed)")
		} else if u.failures >= u.threshold {
			u.state = stateOpen
			log.Warn().Str("proxy", url).Int("failures", u.failures).Msg("circuit opened")
		}
		break
	}
}
