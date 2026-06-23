#!/bin/bash
set -e

# Setup clean environment
WORKSPACE="test-data/telemetry"
rm -rf "$WORKSPACE"
mkdir -p "$WORKSPACE"

echo "[INFO] Building Cortex binary..."
make build

# Create training log (healthy logs)
echo "[INFO] Creating training log..."
cat <<EOF > "$WORKSPACE/training.log"
[INFO] 2026-06-23 12:00:01 - User connection established from IP 192.168.1.50
[INFO] 2026-06-23 12:00:02 - Query executed successfully on database 'orders'
[INFO] 2026-06-23 12:00:03 - Cache hit for key 'product_list_v2'
[INFO] 2026-06-23 12:00:04 - User connection established from IP 192.168.1.51
[INFO] 2026-06-23 12:00:05 - Query executed successfully on database 'orders'
EOF

# Create live log
echo "[INFO] Creating empty live log..."
touch "$WORKSPACE/live.log"

echo "[INFO] Training baseline model..."
./cortex train --file "$WORKSPACE/training.log" --clusters 0

echo "[INFO] Starting Dummy SaaS server in background..."
go run scripts/dummy_saas.go > "$WORKSPACE/saas.log" 2>&1 &
SAAS_PID=$!

# Wait for gRPC server to start listening
echo "[INFO] Waiting for Dummy SaaS to start..."
for i in {1..10}; do
  if grep -q "Listening on :50051" "$WORKSPACE/saas.log" 2>/dev/null; then
    echo "[INFO] Dummy SaaS is ready."
    break
  fi
  sleep 0.5
done

# Run inference with SaaS telemetry enabled
echo "[INFO] Running Cortex inference..."
# We use threshold 0.9 to guarantee the live log triggers an anomaly
./cortex infer --file "$WORKSPACE/live.log" --threshold 0.9 \
  --saas-endpoint "localhost:50051" --saas-token "test-saas-token-12345" --verbose > "$WORKSPACE/infer.log" 2>&1 &
INFER_PID=$!

# Wait for inference to start monitoring
sleep 1.5

# Append anomalous line to live log
echo "[INFO] Injecting anomalous log line..."
echo "[CRITICAL] 2026-06-23 12:05:00 - SQL Injection vulnerability detected from IP 45.33.22.11" >> "$WORKSPACE/live.log"

# Let it process the line and send telemetry
echo "[INFO] Waiting for telemetry transmission..."
sleep 2.5

# Kill processes
echo "[INFO] Cleaning up test processes..."
kill $SAAS_PID 2>/dev/null || true
kill $INFER_PID 2>/dev/null || true

# Assertions
echo "[INFO] Performing assertions..."
JSON_FILE="test-data/telemetry_received.json"

if [ ! -f "$JSON_FILE" ]; then
  echo "❌ FAIL: Telemetry JSON file was not created!"
  echo "--- SaaS logs ---"
  cat "$WORKSPACE/saas.log"
  echo "--- Infer logs ---"
  cat "$WORKSPACE/infer.log"
  exit 1
fi

# Parse json values (basic parsing using grep since jq might not be installed)
TOKEN=$(grep -o '"token": "[^"]*' "$JSON_FILE" | cut -d'"' -f4)
VECTOR_LEN=$(grep -o '"vector_len": [0-9]*' "$JSON_FILE" | awk '{print $2}')
ANOMALY_SCORE=$(grep -o '"anomaly_score": [0-9.]*' "$JSON_FILE" | awk '{print $2}')

echo "--- Telemetry received ---"
cat "$JSON_FILE"
echo "--------------------------"

if [ "$TOKEN" != "test-saas-token-12345" ]; then
  echo "❌ FAIL: Token mismatch! Got '$TOKEN', expected 'test-saas-token-12345'"
  exit 1
fi

if [ "$VECTOR_LEN" -ne 1256 ]; then
  echo "❌ FAIL: Vector length is $VECTOR_LEN, expected 1256 bytes!"
  exit 1
fi

if (( $(echo "$ANOMALY_SCORE < 0.9" | bc -l) )); then
  echo "✅ OK: Anomaly score $ANOMALY_SCORE reported correctly (below 0.9 threshold)"
else
  echo "❌ FAIL: Anomaly score $ANOMALY_SCORE is invalid"
  exit 1
fi

echo "🎉 PASS: Telemetry gRPC integration validated successfully!"
rm -rf "$WORKSPACE"
rm -f "$JSON_FILE"
