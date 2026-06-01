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

func (proxy *Proxy) isAvailable() bool {
	return time.Now().After(proxy.cooldownUntil)
}

type Manager struct {
	mutex            sync.RWMutex
	proxies          []*Proxy
	consecutiveFails map[string]int
	currentIndex     int
}

func NewManager(proxyURLs []string) *Manager {
	proxies := make([]*Proxy, 0, len(proxyURLs))
	for _, proxyURL := range proxyURLs {
		proxies = append(proxies, &Proxy{URL: proxyURL})
	}

	return &Manager{
		proxies:          proxies,
		consecutiveFails: make(map[string]int),
	}
}

func (manager *Manager) Pick() *Proxy {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	if len(manager.proxies) > 0 {
		for range manager.proxies {
			manager.currentIndex = (manager.currentIndex + 1) % len(manager.proxies)
			proxy := manager.proxies[manager.currentIndex]
			if proxy.isAvailable() {
				return proxy
			}
		}
	}

	log.Warn().Msg("no available proxy")
	return nil
}

func (manager *Manager) getProxy(proxyURL string) *Proxy {
	for _, proxy := range manager.proxies {
		if proxy.URL == proxyURL {
			return proxy
		}
	}
	return nil
}

func (manager *Manager) RecordSuccess(proxyURL string) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	manager.consecutiveFails[proxyURL] = 0
}

func (manager *Manager) RecordFailure(proxyURL string) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	manager.consecutiveFails[proxyURL]++
	if manager.consecutiveFails[proxyURL] >= consecutiveFailThreshold {
		proxy := manager.getProxy(proxyURL)
		if proxy != nil {
			proxy.cooldownUntil = time.Now().Add(cooldownDuration)
			manager.consecutiveFails[proxy.URL] = 0
		}
	}
}
