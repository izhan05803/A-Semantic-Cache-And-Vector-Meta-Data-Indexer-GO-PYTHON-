# Semantic Cache & Vector Metadata Indexer

> Go + Python pipeline with ChromaDB, Gemini LLM, Prometheus metrics, OpenTelemetry tracing, and Kubernetes — built across 5 production-hardening engineering phases.

[![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go)](https://golang.org)
[![Python](https://img.shields.io/badge/Python-3.12-FFD43B?logo=python)](https://python.org)
[![gRPC](https://img.shields.io/badge/gRPC-proto-00E5FF)](https://grpc.io)
[![ChromaDB](https://img.shields.io/badge/ChromaDB-vector-00E5FF)](https://trychroma.com)
[![Gemini](https://img.shields.io/badge/Gemini-Flash--Lite-F5A623)](https://deepmind.google/gemini)
[![K8s](https://img.shields.io/badge/Kubernetes-manifests-326CE5?logo=kubernetes)](https://kubernetes.io)
[![Prometheus](https://img.shields.io/badge/Prometheus-metrics-E6522C?logo=prometheus)](https://prometheus.io)
[![OTel](https://img.shields.io/badge/OpenTelemetry-traces-00E5FF?logo=opentelemetry)](https://opentelemetry.io)

---

## Interactive Portfolio

A live, browser-based showcase of the entire system — including a **spinning ASCII donut** terminal animation, interactive query demo with real-time request flow visualization, and a full metrics dashboard.

👉 **[Open project.html →](https://izhan05803.github.io/A-Semantic-Cache-And-Vector-Meta-Data-Indexer-GO-PYTHON-/project.html)**

| Feature | Description |
|---|---|
| 🍩 Spinning ASCII Donut | Real-time terminal art rendering the classic donut.c algorithm |
| ▶ Live Query Demo | Simulate cache hit/miss with animated request flow through each service |
| 📊 Metrics Dashboard | 8 real-time Prometheus metrics: hit rate, latency, circuit breaker state, etc. |
| 🏗️ Architecture SVG | Full system diagram with component interactions and security layers |
| 🧱 5 Engineering Phases | Complete build log from observability to Kubernetes deployment |

---

## Table of Contents

- [Architecture](#architecture)
- [Features](#features)
- [Metrics](#metrics)
- [Engineering Phases](#engineering-phases)
- [Getting Started](#getting-started)
  - [Local Run](#local-run)
  - [Docker](#docker)
- [API Reference](#api-reference)
- [Configuration](#configuration)
- [CI/CD](#cicd)
- [Kubernetes](#kubernetes)

---

## Architecture

![System Architecture Diagram](assets/architecture.svg)

**Request flow:**
1. Client sends query to **Go Proxy** → input validation → rate limit check
2. Proxy forwards via **gRPC** to **Python Indexer**
3. Indexer embeds query and searches **ChromaDB**
   - **Cache hit** (similarity ≥ threshold) → return cached response
   - **Cache miss** → call **Gemini Flash-Lite** → store result + vector → return
4. Response returned to client with cache status and similarity score

---

## Features

| Layer | Capability |
|---|---|
| **Semantic Cache** | Cosine similarity over vector embeddings; semantically equivalent queries hit the same cache entry |
| **Vector Indexer** | Sentence-Transformers embedding + ChromaDB storage with metadata |
| **Go Proxy** | HTTP/gRPC bridge, rate limiter (per-IP token bucket), circuit breaker, retry with jitter |
| **Python Indexer** | Pydantic Settings, auth interceptor, gRPC health probe, graceful shutdown |
| **Observability** | Structured logging (`log/slog` + Python logging), `/healthz`, gRPC Health/Check |
| **Metrics** | 6 Go + 5 Python Prometheus metrics on separate ports |
| **Tracing** | OpenTelemetry OTLP exporter → Jaeger (env-var configured) |
| **Deployment** | Multi-stage Dockerfiles, docker-compose, GitHub Actions CI/CD, K8s manifests |
| **Portfolio UI** | `project.html` — interactive demo with live metrics and ASCII donut animation |

---

## Metrics

### Go Proxy (`:8080/metrics`)

| Metric | Type | Description |
|---|---|---|
| `proxy_requests_total` | Counter | Total HTTP requests |
| `proxy_cache_hits_total` | Counter | Semantic cache hits |
| `proxy_cache_misses_total` | Counter | Semantic cache misses |
| `proxy_request_duration_ms` | Histogram | Request latency |
| `proxy_gemini_request_duration_ms` | Histogram | Gemini API latency |
| `proxy_circuit_breaker_state` | Gauge | 0=Closed, 1=Open, 2=Half-Open |

### Python Indexer (`:8000/`)

| Metric | Type | Description |
|---|---|---|
| `indexer_requests_total` | Counter | Total gRPC requests |
| `indexer_cache_hits_total` | Counter | Indexer-level cache hits |
| `indexer_cache_misses_total` | Counter | Indexer-level cache misses |
| `indexer_cache_updates_total` | Counter | Embeddings stored |
| `indexer_request_duration_seconds` | Histogram | gRPC request duration |

---

## Engineering Phases

Each phase adds a critical production-hardening layer:

```
Phase 1: Observability & Reliability
  ├── Python Indexer — structured logging
  ├── Go Proxy — structured logging (log/slog)
  ├── Health check endpoint (/healthz)
  ├── Graceful shutdown (SIGTERM handling)
  └── gRPC health probe

Phase 2: Configuration & Portability
  ├── Pydantic Settings for environment config
  ├── Multi-stage Dockerfiles (Go + Python)
  ├── docker-compose.yml with Jaeger
  └── Portable ChromaDB volume mounting

Phase 3: Security & Input Validation
  ├── Input validation (Go proxy handler)
  ├── gRPC auth interceptor (server-side)
  └── Per-IP token-bucket rate limiter

Phase 4: Resilience & Performance
  ├── 3-state circuit breaker
  ├── Exponential backoff with jitter
  └── gRPC keepalive & connection pooling

Phase 5: Deployment & Monitoring
  ├── Prometheus metrics (6 Go + 5 Python)
  ├── OpenTelemetry + Jaeger tracing
  ├── GitHub Actions CI/CD pipeline
  └── Kubernetes manifests (deployments, services, PVC)
```

---

## Getting Started

### Prerequisites

- Go ≥ 1.22
- Python ≥ 3.9
- `GOOGLE_API_KEY` at `/Users/Izhan/.env`
- `INDEXER_API_KEY` in project `.env`

### Local Run

```bash
# 1. Start the Python Indexer
cd indexer
pip install -r requirements.txt
python3 server.py &
# gRPC on :50051 · metrics on :8000

# 2. Start the Go Proxy
cd ../proxy
export INDEXER_API_KEY="your-key"
go run ./cmd/server/ &
# HTTP on :8080 · metrics on :8080/metrics

# 3. Test
curl localhost:8080/healthz
curl -X POST localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"capital of France"}'
```

### Docker

```bash
docker compose up --build
# Indexer :50051 · Proxy :8080 · Jaeger UI :16686
```

---

## API Reference

### Go Proxy (HTTP)

| Endpoint | Method | Description |
|---|---|---|
| `/chat` | POST | Send a query (returns cache hit/miss + score) |
| `/healthz` | GET | Health check (`{"status":"ok"}`) |
| `/metrics` | GET | Prometheus metrics |

**`POST /chat` example:**
```json
{"message": "what is the capital of France?"}
```

**Response (cache hit):**
```json
{
  "cached": true,
  "score": 0.97,
  "response": "The capital of France is Paris."
}
```

### Python Indexer (gRPC)

Defined in `indexer/proto/` — `CacheService` with `Check` and `Store` RPCs. Auth via `indexer-api-key` metadata header.

---

## Configuration

| Variable | Component | Default |
|---|---|---|
| `INDEXER_PORT` | Indexer | `50051` |
| `METRICS_PORT` | Indexer | `8000` |
| `CHROMA_DB_PATH` | Indexer | `data/chroma_db` |
| `EMBEDDING_MODEL` | Indexer | `all-MiniLM-L6-v2` |
| `INDEXER_API_KEY` | Indexer | Required |
| `GOOGLE_API_KEY` | Proxy | Required (at `/Users/Izhan/.env`) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Both | `http://localhost:4317` |

---

## CI/CD

GitHub Actions workflow (`.github/workflows/ci.yml`):

- **Lint** — `golangci-lint` (Go) + `ruff` (Python)
- **Type check** — `go vet` + `mypy`
- **Test** — `go test ./...` + `pytest`
- **Build** — multi-stage Docker images
- **Deploy** — push to container registry + update K8s manifests

---

## Kubernetes

Manifests in `k8s/`:

| Resource | File |
|---|---|
| Namespace | `00-namespace.yaml` |
| Indexer Deployment + Service | `indexer.yaml` |
| Proxy Deployment + Service | `proxy.yaml` |
| ConfigMap | `configmap.yaml` |
| PVC (ChromaDB) | `pvc.yaml` |
| Secrets | `secrets.yaml` |
| Jaeger All-in-One | `jaeger.yaml` |

```bash
kubectl apply -f k8s/
```

---

## Project Structure

```
.
├── indexer/              # Python — embedding, gRPC server, ChromaDB
│   ├── server.py         # gRPC server + CacheService + health probe
│   ├── indexer.py        # Embedding + vector search logic
│   ├── metrics.py        # Prometheus metrics
│   ├── tracing.py        # OpenTelemetry init
│   ├── config.py         # Pydantic Settings
│   ├── Dockerfile
│   └── requirements.txt
├── proxy/                # Go — HTTP/gRPC bridge, rate limiter, circuit breaker
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── proxy/        # Handler, metrics, rate limiter
│   │   ├── cache/        # gRPC client + auth interceptor
│   │   ├── circuitbreaker/
│   │   ├── retry/        # Exponential backoff with jitter
│   │   └── tracing/
│   └── Dockerfile
├── k8s/                  # Kubernetes manifests
├── .github/workflows/    # CI/CD
├── docker-compose.yml
└── project.html          # Interactive portfolio page
```

---

## Author

**izhan05803** — [github.com/izhan05803](https://github.com/izhan05803)
