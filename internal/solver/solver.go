package solver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/william1nguyen/midproxy/internal/store"
)

// Solver calls external puppeteer browser nodes to solve Cloudflare challenges.
// Each node runs a real browser pool (puppeteer).
// Nodes are selected round-robin as a simple load balancer.
type Solver struct {
	nodes  []string
	mu     sync.Mutex
	index  int
	client *http.Client
}

func New(nodes []string, timeout time.Duration) *Solver {
	return &Solver{
		nodes:  nodes,
		client: &http.Client{Timeout: timeout},
	}
}

func (s *Solver) pickNode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.nodes) == 0 {
		return ""
	}
	s.index = (s.index + 1) % len(s.nodes)
	return s.nodes[s.index]
}

type solveRequest struct {
	URL   string `json:"url"`
	Proxy string `json:"proxy"`
}

type solveResponse struct {
	Cookies []store.Cookie `json:"cookies"`
}

func (s *Solver) Solve(ctx context.Context, targetURL, proxyURL string) ([]store.Cookie, error) {
	node := s.pickNode()
	if node == "" {
		return nil, fmt.Errorf("no solver nodes available")
	}

	payload, _ := json.Marshal(solveRequest{URL: targetURL, Proxy: proxyURL})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, node+"/solve", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("solver request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("solver returned %d", resp.StatusCode)
	}

	var result solveResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode solver response: %w", err)
	}
	return result.Cookies, nil
}
