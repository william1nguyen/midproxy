# Midproxy

HTTP/HTTPS middleman proxy that automatically detects and solves Cloudflare challenges using browser automation.

## Architecture

<p align="center">
  <img src="docs/architecture.svg" alt="System Architecture" width="100%"/>
</p>

## Request Flow

<p align="center">
  <img src="docs/request-flow.svg" alt="Request Flow" width="100%"/>
</p>

## System Design

### Proxy (Go)

- **MITM interception** — dynamically generates TLS certificates per host to inspect HTTPS traffic
- **Upstream proxy pool** — round-robin selection with circuit breaker (CLOSED → OPEN → HALF_OPEN) per proxy
- **Retry with backoff** — failed requests auto-retry with exponential backoff and alternate proxies; CF challenges with stale cookies retry before re-solving
- **Rate limiting** — per-client-IP fixed window using Redis counters
- **Response caching** — cacheable GET 200 responses cached in Redis with configurable TTL

### Solver (TypeScript)

- **Browser pool** — manages multiple Chrome instances with tab reuse and idle cleanup
- **Cloudflare bypass** — uses Puppeteer to load pages and extract `cf_clearance` cookies
- **Redis Streams** — consumer group pattern (`XREADGROUP`/`XACK`) for reliable job processing, scalable to multiple workers
- **Dead letter queue** — failed jobs retry N times, then move to `queue:dead` for inspection

### Communication

```
Go Proxy ──XADD──→ Redis Stream (stream:solve) ──XREADGROUP──→ TS Solver
                          ↑                                          │
                          └── cookies:{domain} ←── LPUSH ────────────┘
```

- **Job deduplication** — `SET NX solving:{domain}` prevents duplicate solves; lock value = job ID for stale detection
- **Dynamic Retry-After** — reads remaining TTL from solve lock, client knows exactly when to retry

## Tech Stack

| Component | Stack |
| --- | --- |
| Proxy | Go, [tls-client](https://github.com/bogdanfinn/tls-client), zerolog |
| Solver | TypeScript, [puppeteer-real-browser](https://github.com/nicefeel/puppeteer-real-browser), pino, tsup |
| Queue | Redis Streams (consumer groups) |
| Infra | Valkey/Redis, Docker Compose |

## Quick Start

```bash
cp configs/config.example.yaml configs/config.yaml
cp solver/.env.example solver/.env
docker compose -p midproxy up -d
curl -k -x http://localhost:8080 https://example.com
```

<details>
<summary><b>Local development</b></summary>

```bash
make docker-up        # start redis
make dev              # start proxy
make solver-dev       # start solver (new terminal)
```

</details>

## Configuration

<details>
<summary><b>Proxy</b> — <code>configs/config.yaml</code></summary>

```yaml
port: 8080
proxies:
  - http://user:pass@proxy1:8080

solver:
  enabled: true
  timeout: 180s

redis:
  address: localhost:6379
  password: ""
  db: 0

fetch:
  timeout: 30s
  max_retries: 3
  retry_base_delay: 1s
  retry_max_delay: 8s

circuit:
  failure_threshold: 5
  reset_timeout: 30s

cache:
  enabled: true
  ttl: 5m

rate_limit:
  max_rps: 5
```

</details>

<details>
<summary><b>Solver</b> — <code>solver/.env</code></summary>

| Variable | Default | Description |
| --- | --- | --- |
| `REDIS_URL` | — | Redis connection string |
| `PROXY_LIST` | — | Comma-separated upstream proxies |
| `HEADLESS` | `false` | Run browser in headless mode |
| `MAX_BROWSERS` | `3` | Max browser instances |
| `MAX_TABS` | `3` | Max tabs per browser |
| `MAX_JOB_RETRIES` | `3` | Retries before dead letter queue |

</details>

## Development

```bash
make lint             # golangci-lint + biome
make test             # go tests + vitest
lefthook install      # pre-commit: lint, pre-push: tests
```
