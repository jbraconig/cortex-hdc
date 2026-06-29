# Cortex-HDC: Log Anomaly Detection with Hyperdimensional Computing

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

**Cortex-HDC** is an ultra-lightweight, high-performance engine written in Go that uses **Hyperdimensional Computing (HDC)** for real-time log anomaly detection.

Unlike traditional systems based on regular expressions (RegEx) or massive clusters like Elasticsearch, Cortex-HDC creates mathematical "fingerprints" of logs using high-dimensional vectors (10,000 bits) to learn the system's "healthy state". By ignoring chaotic identifiers (such as UUIDs, dates, and response times), it detects drastic structural deviations instantly, using very few megabytes of memory.

## Features

- 🧠 **Mathematics instead of RegEx**: Immune to minor text variations; inherently ignores random IDs.
- 🌱 **O(1) Streaming Training**: Trains models on massive log files (millions of lines) with a flat, constant memory footprint using Mini-Batch K-Means and incremental bundling.
- ⚡ **Lock-Free / Batched Decay**: Eliminates mutex lock contention in worker pools during baseline updates via a dedicated asynchronous queue worker.
- 🛡️ **Context-Aware Backpressure**: Halts input parsing and blocks log ingestion under worker saturation, avoiding memory spikes while maintaining zero-leak context cancellation safety.
- 🧹 **Built-in Noise Filtering**: Built-in zero-allocation timestamp cleaner that automatically filters out standard time/date formats (RFC3339, ISO8601, Syslog, Apache) to focus only on structural log patterns.
- 🔄 **P2P Cluster Sync (Gossip)**: Native multi-node baseline synchronization using `memberlist` (Gossip protocol), eliminating external brokers (like Redis) for clustered setups.
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
./cortex train --file /path/to/your/healthy.log --clusters 3
```

- This process reads the log and trains the engine in streaming mode using a constant memory footprint ($O(1)$ RAM). It vectorizes each line, applies streaming Mini-Batch K-Means clustering (if `--clusters >= 2`) or incremental bundling, and generates mathematical signatures (`Baselines`).
- It automatically performs a statistical pass (Auto-Tuning) to calculate the standard deviation and suggest the optimal `--threshold`.
- The trained model and the suggested threshold will be saved in the persistent database file `cortex.kv`.

### 3. Inference Phase (Real-Time)

Once the model is trained, start the real-time analysis (`tail -f` mode) pointing to your live production log stream. You can monitor multiple files concurrently by separating them with commas.

```bash
./cortex infer --file /var/log/syslog,/var/log/nginx/access.log --workers 8 --threshold 0.65 --decay-rate 0.001 --webhook http://your-alertmanager:9093/api/v2/alerts
```

### 4. Auto Mode (Train + Infer)

If you want the engine to automatically train on the first run if the knowledge base doesn't exist, use the `auto` command. It will scan the directory provided in `--init-logs` (default: `/data/init-logs/`) and train on all log files found before starting inference.

```bash
./cortex auto --file /var/log/syslog,/var/log/mysql.log --init-logs /path/to/baseline/logs/ --clusters 3
```

### Configuration Flags

Below are the detailed flags for each command (`train`, `infer`, `auto`).

#### 1. `train` Command Flags
Used to build the baseline database from a healthy log file.
- `--file` *(required)*: Path to the healthy log file to train on.
- `--clusters` *(optional, default 0)*: Number of baseline clusters. Use `>= 2` to enable Mini-Batch K-Means clustering.
- `--multiline-prefix` *(optional)*: Prefix (literal string or prefix substring) to detect the start of a log line. Subsequent lines not matching this prefix will be buffered as a single multiline log entry.
- `--multiline-max-lines` *(optional, default 5)*: Maximum number of lines to buffer for a single multiline log entry.
- `--date-regex` *(optional)*: Custom regular expression to match and mask timestamps (replaced by `<DATE>`). Note that Cortex-HDC already automatically filters standard log timestamps by default.

#### 2. `infer` Command Flags
Used for real-time anomaly detection.
- `--file` *(required)*: Path to the live log file(s) to monitor. Multiple files can be separated by commas (e.g. `/var/log/syslog,/var/log/nginx.log`).
- `--workers` *(optional, default 4)*: Number of concurrent processing worker goroutines.
- `--threshold` *(optional, default 0.65)*: Mathematical similarity threshold. If anomaly similarity falls below this threshold, alert notifications are triggered. If not specified, the system will use the auto-tuned threshold stored in `cortex.kv`.
- `--webhook` *(optional)*: HTTP POST endpoint where alerts will be dispatched in JSON format.
- `--decay-rate` *(optional, default 0.0)*: Learning rate for Memory Decay (e.g. `0.001`), enabling online adaptation of the baseline towards incoming healthy logs.
- `--p2p` *(optional, default false)*: Enable P2P baseline synchronization across nodes.
- `--p2p-bind` *(optional, default 7946)*: Port for P2P gossip communication.
- `--p2p-join` *(optional)*: Seed node addresses to join (comma-separated, e.g. `10.0.0.1:7946,10.0.0.2:7946`).
- `--saas-endpoint` *(optional)*: SaaS Control Plane gRPC endpoint (e.g., `cloud.yourcompany.com:50051`).
- `--saas-token` *(optional)*: Authentication token for SaaS Control Plane.
- `--heartbeat-interval` *(optional, default 60)*: Telemetry heartbeat interval in seconds.
- `--send-raw-logs` *(optional, default false)*: Send unmasked raw logs to the SaaS portal during anomaly reporting (disables privacy mode).
- `--multiline-prefix` *(optional)*: Prefix to detect the start of log lines (e.g., `2026-`).
- `--multiline-timeout` *(optional, default 500)*: Timeout in milliseconds to flush the multiline buffer if no new log line arrives.
- `--multiline-max-lines` *(optional, default 5)*: Maximum lines to buffer per multiline entry.
- `--date-regex` *(optional)*: Custom regular expression to match and mask timestamps.
- `--verbose` *(optional, default false)*: Print all logs (`[OK]` and `[ANOMALY]`) to stdout instead of only alerting on anomalies.

#### 3. `auto` Command Flags
Combines both training and inference. Trains from initial directory if the database is missing.
- `--file` *(required)*: Path to the live log file(s) to monitor (comma-separated).
- `--init-logs` *(optional, default `/data/init-logs/`)*: Directory containing baseline healthy logs for auto-training.
- `--workers` *(optional, default 4)*: Number of concurrent processing workers.
- `--clusters` *(optional, default 0)*: Number of baseline clusters to train (using Mini-Batch K-Means).
- `--threshold` *(optional, default 0.65)*: Mathematical similarity threshold.
- `--webhook` *(optional)*: HTTP POST endpoint for alerts.
- `--decay-rate` *(optional, default 0.0)*: Learning rate for Memory Decay.
- `--multiline-prefix` *(optional)*: Prefix to detect the start of log lines.
- `--multiline-timeout` *(optional, default 500)*: Timeout in milliseconds to flush the multiline buffer.
- `--multiline-max-lines` *(optional, default 5)*: Maximum lines to buffer per multiline entry.
- `--date-regex` *(optional)*: Custom regular expression to match and mask timestamps.
- `--verbose` *(optional, default false)*: Print all logs to stdout.

---

### SaaS Integration & Heartbeats
When connected to a SaaS Control Plane (via `--saas-endpoint` and authenticated with `--saas-token`), Cortex-HDC starts a background worker that sends periodic "heartbeats" (signals of life). This allows the Control Plane to monitor node health. If the node fails to send a heartbeat within the configured window, the SaaS platform marks the node as `offline` and dispatches alert notifications (email/webhooks).

---

## 🐳 Docker Quick Start

The easiest way to run Cortex-HDC without installing Go is via Docker.

### Running with Docker Compose

An example `docker-compose.yml` is provided in `deploy/docker-compose.yml`.

1. Copy the file or run it from the repository root.
2. It mounts the host's `/var/log` into `/data/logs` inside the container.
3. (Optional) Mount your healthy baseline logs into `./init-logs`. The container will automatically train on these logs on its first boot if `cortex.kv` is missing.
4. Start the engine:

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
  cortex-hdc infer --file /host/logs/syslog,/host/logs/nginx.log --decay-rate 0.001 --verbose
```

---

## Observability & Metrics

Cortex exposes internal metrics via a Prometheus exporter and runtime profiling via Go pprof running on port `9090`.

- **Prometheus Metrics**: Available at `/metrics` (e.g., `http://localhost:9090/metrics`). Tracks total logs processed, anomalies detected, and similarity score distributions.
- **Go pprof Profiling**: Available at `/debug/pprof/` (e.g., `http://localhost:9090/debug/pprof/`). Allows capturing real-time heap, CPU, and goroutine profiles for performance debugging.

---

## ⚡ Performance & Benchmarks

Cortex-HDC is built for maximum throughput with a negligible resource footprint. In a real-world benchmark run processing a stream of **128,000 logs** with 8 concurrent workers, Cortex achieved:

- **Ingestion Throughput**: **~8,600+ logs/second** (parsing, cleaning, vectorizing, and similarity checking).
- **Memory Footprint**: **< 7 MB of active RAM heap** at peak load.
- **Garbage Collection Latency**: Max stop-the-world pause of **0.25 milliseconds** (median pause of **0.06 milliseconds**).

### Running Benchmarks Locally

To execute the benchmark suit on your hardware:

```bash
./scripts/benchmark.sh
```

This script will output the Prometheus raw metrics to `test-data/metrics.txt` and save the Go `pprof` profile to `test-data/heap.pprof`. You can analyze the heap allocations interactively in your browser:

```bash
go tool pprof -http=:8080 test-data/heap.pprof
```

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

## Academic Foundations

The core architecture and mathematical principles of **Cortex-HDC** are heavily inspired by and aligned with recent breakthroughs in applied Hyperdimensional Computing research. If you are interested in the scientific foundation behind this engine, we recommend reviewing the following academic literature:

1. **[D2H-AD: A Hybrid Model Utilizing Hyperdimensional Computing for Advanced Anomaly Detection](https://ieeexplore.ieee.org/document/11455993) (IEEE Access)**
   This study introduces a hybrid anomaly detection framework built upon HDC, demonstrating that fusing distance-based similarity (like our use of Hamming distance) with density-aware encoding significantly improves anomaly detection accuracy while maintaining a lightweight footprint suitable for edge AI.
2. **[ODHD: One-Class Brain-Inspired Hyperdimensional Computing for Outlier Detection](https://dl.acm.org/doi/10.1145/3489517.3530395) (ACM DAC '22)**
   This paper outlines a one-class outlier detection method leveraging the inherent robustness and computational efficiency of HDC. It validates our approach of using highly parallelizable, binary operations (`XOR` and `POPCNT`) to achieve ultra-low latency and resource-efficient processing over traditional machine learning methods.

---

## License

This project is distributed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.
