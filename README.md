# Midproxy

An HTTP/HTTPS middleman proxy that detects and solves Cloudflare challenges automatically.

## Architecture

<p align="center">
  <img src="docs/architecture.svg" alt="System Architecture" width="100%"/>
</p>

## Request Flow

<p align="center">
  <img src="docs/request-flow.svg" alt="Request Flow" width="100%"/>
</p>

## Features

- Automatic Cloudflare challenge detection and solving
- HTTPS interception via MITM with dynamic certificate generation
- Upstream proxy pool with round-robin and auto-cooldown on failures
- Per-domain rate limiting
- Response caching (GET 200)
- Browser pool with multiple instances, tab reuse, and idle cleanup (default: 3 browsers × 3 tabs = 9 concurrent solves)
- Solve job deduplication per domain with Redis lock (`solving:{domain}`)
- Dynamic `Retry-After` header based on remaining solve time
- Stale job detection — solver skips outdated jobs when cookies are re-solved

## Prerequisites

- Go 1.26+
- Node.js 22+ & pnpm
- Docker

## Quick Start

### Docker (recommended)

```bash
cp configs/config.example.yaml configs/config.yaml
cp solver/.env.example solver/.env          # configure proxies & redis
docker compose -p midproxy up -d
curl -k -x http://localhost:8080 https://2captcha.com/demo/cloudflare-turnstile-challenge
```

### Local development

```bash
cp configs/config.example.yaml configs/config.yaml
make docker-up                               # start redis
make dev                                     # start proxy
make solver-dev                              # start solver (new terminal)
curl -k -x http://localhost:8080 https://2captcha.com/demo/cloudflare-turnstile-challenge
```

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
| `IDLE_TIMEOUT` | `300000` | Close idle browsers (ms) |
| `CLEARANCE_TIMEOUT` | `30000` | Wait for cf_clearance (ms) |
| `NAVIGATION_TIMEOUT` | `60000` | Page load timeout (ms) |

</details>

## Development

### Lint

```bash
make lint            # run all linters (golangci-lint + biome)
make lint-proxy      # go only
make lint-solver     # solver only
```

### Test

```bash
make test            # unit + solver tests
make test-proxy      # go tests with race detector + coverage
make test-solver     # solver vitest
```

### Pre-commit hooks

Managed by [Lefthook](https://github.com/evilmartians/lefthook). Install with `lefthook install`.

- **pre-commit**: lint + format (only for changed files)
- **pre-push**: unit tests

### Build

```bash
make build                    # go binary → bin/proxy
cd solver && pnpm build       # tsup bundle → dist/index.js
```

## Tech Stack

| Component | Stack |
| --- | --- |
| Proxy | Go, [tls-client](https://github.com/bogdanfinn/tls-client), zerolog |
| Solver | TypeScript, [puppeteer-real-browser](https://github.com/nicefeel/puppeteer-real-browser), pino, tsup |
| Lint | golangci-lint, [Biome](https://biomejs.dev) |
| Infra | Valkey/Redis, Docker Compose |
