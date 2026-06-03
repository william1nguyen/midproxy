package proxy

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	consecutiveFailThreshold = 3
	cooldownDuration         = 5 * time.Minute
)

type Proxy struct {
	URL           string
	cooldownUntil time.Time
}

func (p *Proxy) isAvailable() bool {
	return time.Now().After(p.cooldownUntil)
}

type Manager struct {
	mutex            sync.Mutex
	proxies          []*Proxy
	consecutiveFails map[string]int
	index            int
}

func NewManager(proxyURLs []string) *Manager {
	proxies := make([]*Proxy, 0, len(proxyURLs))
	for _, u := range proxyURLs {
		proxies = append(proxies, &Proxy{URL: u})
	}

	return &Manager{
		proxies:          proxies,
		consecutiveFails: make(map[string]int),
	}
}

func (m *Manager) Pick() *Proxy {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for range m.proxies {
		m.index = (m.index + 1) % len(m.proxies)
		proxy := m.proxies[m.index]
		if proxy.isAvailable() {
			return proxy
		}
	}

	log.Warn().Msg("no available proxy")
	return nil
}

func (m *Manager) getProxy(proxyURL string) *Proxy {
	for _, p := range m.proxies {
		if p.URL == proxyURL {
			return p
		}
	}
	return nil
}

func (m *Manager) RecordSuccess(proxyURL string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.consecutiveFails[proxyURL] = 0
}

func (m *Manager) RecordFailure(proxyURL string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.consecutiveFails[proxyURL]++
	if m.consecutiveFails[proxyURL] >= consecutiveFailThreshold {
		proxy := m.getProxy(proxyURL)
		if proxy != nil {
			proxy.cooldownUntil = time.Now().Add(cooldownDuration)
			m.consecutiveFails[proxy.URL] = 0
		}
	}
}
