package proxy

import (
	"sync"
	"time"
)

const (
	consecutiveFailThreshold = 3
	cooldownDuration         = 5 * time.Minute
)

type Proxy struct {
	ID            string
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

func NewManager() *Manager {
	return &Manager{
		consecutiveFails: make(map[string]int),
	}
}

func (manager *Manager) Add(id, url string) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	manager.proxies = append(manager.proxies, &Proxy{ID: id, URL: url})
}

func (manager *Manager) Pick() *Proxy {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	for range manager.proxies {
		manager.currentIndex = (manager.currentIndex + 1) % len(manager.proxies)
		proxy := manager.proxies[manager.currentIndex]
		if proxy.isAvailable() {
			return proxy
		}
	}

	return nil
}

func (manager *Manager) getProxy(proxyId string) *Proxy {
	for _, proxy := range manager.proxies {
		if proxy.ID == proxyId {
			return proxy
		}
	}
	return nil
}

func (manager *Manager) RecordSuccess(proxyId string) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	manager.consecutiveFails[proxyId] = 0
}

func (manager *Manager) RecordFailure(proxyId string) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	manager.consecutiveFails[proxyId]++
	if manager.consecutiveFails[proxyId] >= consecutiveFailThreshold {
		proxy := manager.getProxy(proxyId)
		if proxy != nil {
			proxy.cooldownUntil = time.Now().Add(cooldownDuration)
			manager.consecutiveFails[proxyId] = 0
		}
	}
}
