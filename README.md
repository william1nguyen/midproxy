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

- Go 1.22+
- Node.js 18+ & pnpm
- Docker (for Redis)

## Quick Start

```bash
cp configs/config.example.yaml configs/config.yaml  # configure proxies & redis
make docker-up                                       # start redis
make dev                                             # start proxy
make solver-dev                                      # start solver (new terminal)

# 5. Test it
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
  timeout: 90s
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

| Variable               | Default    | Description                      |
| ---------------------- | ---------- | -------------------------------- |
| `REDIS_URL`          | —         | Redis connection string          |
| `PROXY_LIST`         | —         | Comma-separated upstream proxies |
| `MAX_BROWSERS`       | `3`      | Max browser instances            |
| `MAX_TABS`           | `3`      | Max tabs per browser             |
| `IDLE_TIMEOUT`       | `300000` | Close idle browsers (ms)         |
| `CLEARANCE_TIMEOUT`  | `30000`  | Wait for cf_clearance (ms)       |
| `NAVIGATION_TIMEOUT` | `60000`  | Page load timeout (ms)           |

</details>

## Tech Stack

| Component | Stack                                                                                   |
| --------- | --------------------------------------------------------------------------------------- |
| Proxy     | Go,[tls-client](https://github.com/bogdanfinn/tls-client), zerolog                         |
| Solver    | Node.js,[puppeteer-real-browser](https://github.com/nicefeel/puppeteer-real-browser), pino |
| Infra     | Valkey/Redis, Docker Compose                                                            |
