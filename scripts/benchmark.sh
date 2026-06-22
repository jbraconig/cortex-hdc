#!/bin/bash
set -e

# Create a clean temp directory in the project for testing
mkdir -p test-data

LOG_FILE="test-data/benchmark.log"
SOURCE_FILE="test-data/source.log"

echo "Generating synthetic logs in $SOURCE_FILE..."
rm -f "$SOURCE_FILE"
touch "$SOURCE_FILE"
# Generate 100k lines efficiently
for i in {1..1000}; do
  echo "2026-06-22T13:05:33-05:00 INFO [service-worker] Worker $i completed successfully with status=200 duration=45ms" >> "$SOURCE_FILE"
done
# Duplicate to quickly reach 100k
for i in {1..7}; do
  cat "$SOURCE_FILE" >> "$SOURCE_FILE.tmp"
  cat "$SOURCE_FILE.tmp" >> "$SOURCE_FILE"
  rm -f "$SOURCE_FILE.tmp"
done
# Now we have 128,000 lines in $SOURCE_FILE instantly

# Ensure the binary is built
echo "Building cortex binary..."
go build -o cortex cmd/cortex/main.go

# Initialize dummy KB if not present
if [ ! -f "cortex.kv" ]; then
  echo "Training baseline..."
  head -n 1000 "$SOURCE_FILE" > test-data/healthy.log
  ./cortex train --file test-data/healthy.log --clusters 0
fi

# Reset log file
rm -f "$LOG_FILE"
touch "$LOG_FILE"

# Run inference in the background (using auto-tuned threshold from cortex.kv)
echo "Starting inference in background..."
./cortex infer --file "$LOG_FILE" --workers 8 --verbose=false > test-data/cortex.log 2>&1 &
PID=$!

# Give it a couple of seconds to start tailing
sleep 2

# Inject 5 highly anomalous log lines at the beginning of the stream so they are processed immediately
echo "Injecting 5 anomalies into the live stream..."
echo "2026-06-22T13:06:00-05:00 FATAL [database] Connection refused to host 10.0.0.5 after 3 retries" >> "$LOG_FILE"
echo "2026-06-22T13:06:01-05:00 CRITICAL [auth] Brute force attack detected from IP 192.168.1.150 - 50 failed attempts" >> "$LOG_FILE"
echo "2026-06-22T13:06:02-05:00 ERROR [filesystem] Out of disk space on partition /dev/sda1" >> "$LOG_FILE"
echo "2026-06-22T13:06:03-05:00 WARN [kernel] Out of memory: Kill process 9482 (java) score 850 or sacrifice child" >> "$LOG_FILE"
echo "2026-06-22T13:06:04-05:00 ALERT [api] High response latency detected: 15400ms on GET /v1/checkout" >> "$LOG_FILE"

echo "Appending 128,000 healthy logs to trigger processing..."
cat "$SOURCE_FILE" >> "$LOG_FILE"

# Give it time to process
sleep 3

echo "Capturing pprof memory heap..."
curl -s http://localhost:9090/debug/pprof/heap > test-data/heap.pprof

echo "Capturing Prometheus metrics..."
curl -s http://localhost:9090/metrics > test-data/metrics.txt

echo "Shutting down inference..."
kill -SIGTERM $PID || true
wait $PID 2>/dev/null || true

# Parse metrics for summary
PROCESSED=$(grep "cortex_logs_processed_total" test-data/metrics.txt | grep -v "#" | awk '{print $2}' || echo "0")
ANOMALIES=$(grep "cortex_anomalies_detected_total" test-data/metrics.txt | grep -v "#" | awk '{print $2}' || echo "0")

echo "========================================="
echo "Benchmark completed successfully!"
echo "Stats captured during run:"
echo "  - Logs analyzed: $PROCESSED"
echo "  - Anomalies detected: $ANOMALIES"
echo "  - Average throughput: $(echo "scale=2; $PROCESSED / 3" | bc 2>/dev/null || echo "N/A") logs/sec"
echo ""
echo "Pprof heap profile saved to: test-data/heap.pprof"
echo "Prometheus raw metrics saved to: test-data/metrics.txt"
echo ""
echo "To visualize the memory profile in your browser:"
echo "  go tool pprof -http=:8080 test-data/heap.pprof"
echo "========================================="
