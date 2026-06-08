package proxy

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	maxConsecutiveFails = 3
	cooldownDuration    = 5 * time.Minute
)

type upstream struct {
	url           string
	cooldownUntil time.Time
}

type Manager struct {
	mu       sync.Mutex
	upstreams []upstream
	fails    map[string]int
	index    int
}

func NewManager(urls []string) *Manager {
	upstreams := make([]upstream, len(urls))
	for i, u := range urls {
		upstreams[i] = upstream{url: u}
	}
	return &Manager{
		upstreams: upstreams,
		fails:     make(map[string]int),
	}
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
		if time.Now().After(u.cooldownUntil) {
			return u.url
		}
	}

	log.Warn().Msg("no available upstream proxy")
	return ""
}

func (m *Manager) RecordSuccess(url string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fails[url] = 0
}

func (m *Manager) RecordFailure(url string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.fails[url]++
	if m.fails[url] >= maxConsecutiveFails {
		for i := range m.upstreams {
			if m.upstreams[i].url == url {
				m.upstreams[i].cooldownUntil = time.Now().Add(cooldownDuration)
				break
			}
		}
		m.fails[url] = 0
	}
}
