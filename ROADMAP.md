# Production Roadmap: Cortex-HDC

This document details the path forward to take Cortex-HDC from a functional prototype to an Enterprise-ready (production) tool.

## Phase 1: Resilience and Stability (High Priority)

- [x] **Log Rotation Handling**: Replace the current tail simulation with an OS event-based reader (`fsnotify` or `github.com/nxadm/tail`). The engine must survive if the log file is deleted, truncated, or rotated by the operating system.
- [x] **Graceful Shutdown**: Capture OS signals (`SIGINT`, `SIGTERM`). Stop ingestion safely, process in-flight logs within channels, close HTTP connections, and shut down workers in an orderly fashion.
- [x] **Environment Configuration (12-Factor App)**: Migrate from purely CLI flags to configuration using libraries like `viper`, allowing configuration files (`config.yaml`) or environment variables (e.g., `CORTEX_THRESHOLD`, `CORTEX_WORKERS`).

## Phase 2: Observability and Integration (Medium Priority)

- [x] **Live Log Viewer (Terminal/TUI)**: Create a pure terminal-based interface (using libraries like `bubbletea` or plain ANSI colors) where logs can be viewed streaming in real time, instantly coloring anomalies in red along with their similarity percentage, without depending on web browsers.
- [x] **Internal Metrics (Prometheus Endpoint)**: Expose a server on port `9090` with a `/metrics` endpoint to visualize Cortex performance (lines processed per second, RAM consumption, similarity averages).
- [x] **Packaging (Dockerization)**: Create a multi-stage `Dockerfile` to generate an ultra-lightweight base image (`scratch` or `alpine`), reducing the container size to ~15MB.
- [x] **Deployment Manifests**: Generate example `docker-compose.yml` and Kubernetes manifests (`DaemonSet` / `Deployment`) to facilitate plug-and-play installation.

## Phase 3: Mathematical Evolution (Advanced)

- [x] **Baseline Clustering (Avoid Saturation)**: Modify the training process. If logs are highly diverse, a single vector will become saturated with `1`s. Implement an algorithm that partitions the "healthy state" into $K$ baseline vectors depending on the log type detected during training.
- [x] **Auto-Tuning**: Allow the training phase to automatically suggest the ideal threshold by calculating the standard variance of the analyzed vectors.
- [x] **Memory Decay (Gradual Forgetting)**: For constantly evolving systems, implement a mechanism where the Baseline adapts slightly to new healthy logs in production, gradually forgetting very old patterns.

## Phase 4: Optimization and Noise Filtering

- [x] **Timestamp Cleaning (Pre-processing)**: Although HDC tolerates noise, dynamically removing or masking the initial dates/times from log lines before passing them to the `Encoder` will drastically increase accuracy, since the timestamp is pure noise that changes every second.
- [x] **Memory Profiling (pprof) & Benchmarks**: Integrate `net/http/pprof` to monitor in real-time whether Go's Garbage Collector is struggling under extreme loads (e.g., 100,000 logs per second) and optimize `HVector` memory allocation. Publish real benchmarks documenting peak memory, latency p99, and throughput.
- [x] **Distributed Synchronization (Gossip Protocol)**: If you run 50 instances of Cortex on 50 servers, implement a lightweight P2P protocol for them to share and unify their learned Baselines without needing a central database.

## Phase 5: High Throughput and Memory Management (Production Scale)

- [ ] **Streaming / Mini-batch K-Means**: Modify the training phase (`trainer.go`) to avoid accumulating all `HVector`s in memory. Implement streaming or mini-batch K-Means to keep a predictable memory footprint when training with millions of log lines.
- [ ] **Lock-free / Batched Decay Updates**: Optimize lock contention during inference. Currently, `DecayRate > 0` uses a global mutex per healthy log line. Implement batched updates or lock-free structures to avoid bottlenecking the worker pool.
- [ ] **Backpressure and Bounded Channels**: Ensure the `LogReader` channels have strict bounded capacities. Implement drop strategies and expose "dropped_logs" metrics to handle saturation gracefully when ingestion rate exceeds processing speed.
- [ ] **Kubernetes Resource Limits**: Define empirical CPU and Memory `requests` and `limits` in the DaemonSet manifest (`daemonset.yaml`), validating them against the pprof benchmarks to guarantee Cortex doesn't cause OOMKills on nodes.
