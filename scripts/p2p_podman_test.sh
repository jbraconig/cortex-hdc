#!/bin/bash
set -e

# Make sure we run from root directory
cd "$(dirname "$0")/.."

echo "Preparing P2P testing workspace..."
mkdir -p test-data/p2p/node1 test-data/p2p/node2 test-data/p2p/node3

# Ensure we have a trained baseline
if [ ! -f "cortex.kv" ]; then
  echo "cortex.kv not found! Generating one..."
  echo "2026-06-22T13:05:33-05:00 INFO [worker] Healthy log line" > test-data/p2p/healthy.log
  go build -o cortex cmd/cortex/main.go
  ./cortex train --file test-data/p2p/healthy.log --clusters 0
fi

# Copy baseline model to nodes
cp -f cortex.kv test-data/p2p/node1/cortex.kv
cp -f cortex.kv test-data/p2p/node2/cortex.kv
cp -f cortex.kv test-data/p2p/node3/cortex.kv

# Create empty live files
touch test-data/p2p/node1/live.log
touch test-data/p2p/node2/live.log
touch test-data/p2p/node3/live.log

echo "Building container image using Podman..."
podman build -t cortex-hdc .

# Cleanup old containers if they exist
echo "Cleaning up any old P2P containers..."
podman rm -f cortex-node1 cortex-node2 cortex-node3 &>/dev/null || true
podman network rm cortex-net &>/dev/null || true

echo "Creating Podman P2P network..."
podman network create cortex-net

echo "Starting cortex-node1..."
podman run --name cortex-node1 --network cortex-net -d \
  -v ./test-data/p2p/node1:/data:z \
  cortex-hdc infer --file /data/live.log --decay-rate 0.01 --p2p --p2p-bind 7946 --verbose=false

# Wait for node1's listener to be up
sleep 3

echo "Starting cortex-node2..."
podman run --name cortex-node2 --network cortex-net -d \
  -v ./test-data/p2p/node2:/data:z \
  cortex-hdc infer --file /data/live.log --decay-rate 0.01 --p2p --p2p-bind 7946 --p2p-join cortex-node1:7946 --verbose=false

echo "Starting cortex-node3..."
podman run --name cortex-node3 --network cortex-net -d \
  -v ./test-data/p2p/node3:/data:z \
  cortex-hdc infer --file /data/live.log --decay-rate 0.01 --p2p --p2p-bind 7946 --p2p-join cortex-node1:7946 --verbose=false

echo "Waiting for Gossip cluster to settle..."
sleep 5

echo "Injecting a log line to Node 1 (triggers a decay update and broadcast)..."
echo "2026-06-22T13:05:33-05:00 INFO [worker] Healthy log line" >> test-data/p2p/node1/live.log

echo "Waiting for replication..."
sleep 4

echo "=== Node 1 Logs ==="
podman logs cortex-node1

echo "=== Node 2 Logs ==="
podman logs cortex-node2

echo "=== Node 3 Logs ==="
podman logs cortex-node3

echo "=== Cleaning up P2P containers..."
podman rm -f cortex-node1 cortex-node2 cortex-node3
podman network rm cortex-net
echo "Done!"
