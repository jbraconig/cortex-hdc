# Cortex-HDC: Log Anomaly Detection with Hyperdimensional Computing

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Cortex-HDC** is an ultra-lightweight, high-performance engine written in Go that uses **Hyperdimensional Computing (HDC)** for real-time log anomaly detection.

Unlike traditional systems based on regular expressions (RegEx) or massive clusters like Elasticsearch, Cortex-HDC creates mathematical "fingerprints" of logs using high-dimensional vectors (10,000 bits) to learn the system's "healthy state". By ignoring chaotic identifiers (such as UUIDs, dates, and response times), it detects drastic structural deviations instantly, using very few megabytes of memory.

## Features

- 🧠 **Mathematics instead of RegEx**: Immune to minor text variations; inherently ignores random IDs.
- ⚡ **Native Performance**: Leverages Go concurrency (*Worker Pool*) and low-level bit manipulation.
- 🛡️ **Clean Architecture**: Modular, decoupled, and highly testable structure.
- 📦 **Pragmatic Dependencies**: Robust log tailing (compatible with log rotation via `nxadm/tail`) and advanced configuration management with `viper`, keeping the core HDC engine and HTTP notifications on the Go standard library.
- 🔔 **Universal Alerts**: Emits JSON webhooks compatible with standard systems like Prometheus Alertmanager, Grafana, or Slack.

---

## Project Structure (Clean Architecture)

```text
cortex-hdc/
├── cmd/
│   └── cortex/           # CLI entry point
├── internal/
│   ├── config/           # Environment variables and Viper configuration
│   ├── domain/           # Entities (HVector, KnowledgeBase) and interfaces (Ports)
│   ├── usecase/          # Application logic (Training and Inference)
│   └── infrastructure/   # Implementations (Log reader, HDC Encoder, Storage, Notifier)
├── Makefile              # Helper commands for building and running
└── cortex.kv             # (Generated) Compact trained model database
```

---

## Quick Start Guide

### 1. Build the Engine

```bash
make build
```

This will generate a native binary named `cortex` in the root of the project.

### 2. Training Phase (Baseline)

For the engine to know what an anomaly is, it must first learn what is "normal". Provide a massive log file containing the healthy (typical) behavior of your system.

```bash
./cortex train --file /path/to/your/healthy.log
```

- This process reads the log, vectorizes each line, combines them (`Bundle`), and generates a master signature (`Baseline`).
- The trained model will be saved in the persistent database file `cortex.kv`.

### 3. Inference Phase (Real-Time)

Once the model is trained, start the real-time analysis (`tail -f` mode) pointing to your live production log stream.

```bash
./cortex infer --file /var/log/syslog --workers 8 --threshold 0.65 --webhook http://your-alertmanager:9093/api/v2/alerts
```

#### Inference Flags

- `--file`: The path to the live log file to monitor.
- `--workers` *(optional, default 4)*: Number of concurrent goroutines assigned to process log lines simultaneously.
- `--threshold` *(optional, default 0.65)*: Minimum mathematical similarity level required (0.0 to 1.0). If a log line's similarity to the healthy signature is below this threshold, it is treated as an anomaly.
- `--webhook` *(optional)*: HTTP POST endpoint (JSON format) where anomaly alerts will be dispatched.
- `--verbose` *(optional)*: Prints all log lines, not just anomalies. Normal lines in gray, anomalies in red.

---

## 🐳 Docker Quick Start

The easiest way to run Cortex-HDC without installing Go is via Docker.

### Running with Docker Compose

An example `docker-compose.yml` is provided in `deploy/docker-compose.yml`.

1. Copy the file or run it from the repository root.
2. It mounts the host's `/var/log` into `/data/logs` inside the container.
3. Start the engine:

```bash
docker compose up -d
```

### Building the Image Locally

```bash
docker build -t cortex-hdc .
```

### Running Inference via Docker CLI

```bash
docker run --rm -d \
  -v /var/log:/host/logs:ro \
  -v $(pwd)/cortex.kv:/data/cortex.kv \
  -p 9090:9090 \
  cortex-hdc infer --file /host/logs/syslog --verbose
```

---

## Observability & Metrics

Cortex exposes internal metrics via a Prometheus exporter running on port `9090` at the `/metrics` endpoint.
It tracks the total logs processed, anomalies detected, and memory consumption.

---

## Alert Integration (Webhook Payload)

When similarity falls below the threshold, Cortex will send a `POST` request with the following JSON structure (ideal for integration with other observability pipelines):

```json
{
  "timestamp": "2026-06-08T10:00:11Z",
  "level": "CRITICAL",
  "message": "Anomaly detected in logs by HDC engine",
  "similarity": 0.6397,
  "log_line": "2026-06-08T10:00:11Z FATAL [db] Connection refused 192.168.1.100 pool=5 error=timeout"
}
```

---

## Useful Makefile Commands

- `make build`: Compiles the `cortex` binary.
- `make train`: Runs a quick training using `train.log`.
- `make infer`: Runs a live inference using `live.log`.
- `make test`: Runs unit tests.
- `make clean`: Deletes the generated binaries and the `.kv` database file.

---

## License

This project is distributed under the MIT license. See the [LICENSE](LICENSE) file for details.
