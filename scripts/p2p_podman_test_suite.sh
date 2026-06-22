#!/bin/bash
set -eo pipefail

# Make sure we run from root directory
cd "$(dirname "$0")/.."

# ANSI color codes for premium visual outputs
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO] $(date '+%Y-%m-%d %H:%M:%S') - $1${NC}"
}

log_success() {
    echo -e "${GREEN}[PASS] $(date '+%Y-%m-%d %H:%M:%S') - $1${NC}"
}

log_warn() {
    echo -e "${YELLOW}[WARN] $(date '+%Y-%m-%d %H:%M:%S') - $1${NC}"
}

log_error() {
    echo -e "${RED}[FAIL] $(date '+%Y-%m-%d %H:%M:%S') - $1${NC}"
}

# Global container ID and log streamer PID storage
NODE1_ID=""
NODE2_ID=""
NODE3_ID=""
NODE4_ID=""
NODE1_PID=""
NODE2_PID=""
NODE3_PID=""
NODE4_PID=""
SETUP_PHASE=true

# --- Cleanup function ---
dump_logs() {
    log_info "Dumping container logs to test-data/p2p/node*.log..."
    if [ -n "$NODE1_ID" ]; then podman logs "$NODE1_ID" > test-data/p2p/node1.log 2>&1 || true; else podman logs cortex-node1 > test-data/p2p/node1.log 2>&1 || true; fi
    if [ -n "$NODE2_ID" ]; then podman logs "$NODE2_ID" > test-data/p2p/node2.log 2>&1 || true; else podman logs cortex-node2 > test-data/p2p/node2.log 2>&1 || true; fi
    if [ -n "$NODE3_ID" ]; then podman logs "$NODE3_ID" > test-data/p2p/node3.log 2>&1 || true; else podman logs cortex-node3 > test-data/p2p/node3.log 2>&1 || true; fi
    if [ -n "$NODE4_ID" ]; then podman logs "$NODE4_ID" > test-data/p2p/node4.log 2>&1 || true; else podman logs cortex-node4 > test-data/p2p/node4.log 2>&1 || true; fi
}

cleanup() {
    # Kill background log streamers if running
    for pid in "$NODE1_PID" "$NODE2_PID" "$NODE3_PID" "$NODE4_PID"; do
        if [ -n "$pid" ]; then
            kill "$pid" &>/dev/null || true
        fi
    done

    if [ "$SETUP_PHASE" = "false" ]; then
        dump_logs
    fi
    log_info "Cleaning up containers and network..."
    local ids_to_remove=""
    for id in "$NODE1_ID" "$NODE2_ID" "$NODE3_ID" "$NODE4_ID"; do
        if [ -n "$id" ]; then
            ids_to_remove="$ids_to_remove $id"
        fi
    done
    if [ -n "$ids_to_remove" ]; then
        podman rm -f $ids_to_remove &>/dev/null || true
    fi
    podman rm -f cortex-node1 cortex-node2 cortex-node3 cortex-node4 &>/dev/null || true
    podman network rm cortex-net &>/dev/null || true
    log_info "Cleanup complete."
}
trap cleanup EXIT

# 1. SETUP
log_info "Setting up test workspace..."
mkdir -p test-data/p2p/node1 test-data/p2p/node2 test-data/p2p/node3 test-data/p2p/node4
rm -f test-data/p2p/node*/cortex.log

# Ensure we have a trained baseline model
if [ ! -f "cortex.kv" ]; then
    log_warn "cortex.kv not found! Generating baseline model..."
    echo "2026-06-22T13:05:33-05:00 INFO [worker] Healthy log line" > test-data/p2p/healthy.log
    # Build local binary to train
    go build -o cortex cmd/cortex/main.go
    ./cortex train --file test-data/p2p/healthy.log --clusters 0
fi

# Copy baseline model to nodes
cp -f cortex.kv test-data/p2p/node1/cortex.kv
cp -f cortex.kv test-data/p2p/node2/cortex.kv
cp -f cortex.kv test-data/p2p/node3/cortex.kv
cp -f cortex.kv test-data/p2p/node4/cortex.kv

# Create empty live files
touch test-data/p2p/node1/live.log
touch test-data/p2p/node2/live.log
touch test-data/p2p/node3/live.log
touch test-data/p2p/node4/live.log

log_info "Building Podman image 'cortex-hdc'..."
podman build -t cortex-hdc .

# Remove any old traces before starting
cleanup

# Finish setup phase, enabling full logging on exit
SETUP_PHASE=false

log_info "Creating network 'cortex-net'..."
podman network create cortex-net

# Helper function to start background log streamer on the host
start_streamer() {
    local node_id=$1
    local log_file=$2
    mkdir -p "$(dirname "$log_file")"
    touch "$log_file"
    podman logs --follow "$node_id" > "$log_file" 2>&1 &
    echo $!
}

# Helper function to poll logs for a pattern (first checking local host-streamed log, falling back to daemon logs)
wait_for_log() {
    local node_identifier=$1
    local log_file=$2
    local pattern=$3
    local timeout=$4
    local elapsed=0
    while [ $elapsed -lt $timeout ]; do
        # 1. Try reading the host-streamed log file (zero podman overhead)
        if [ -f "$log_file" ] && grep -q "$pattern" "$log_file"; then
            return 0
        fi
        sleep 1
        elapsed=$((elapsed + 1))
    done
    # Last-ditch check using direct podman logs if streamer had issues
    if [ -n "$node_identifier" ] && podman logs "$node_identifier" 2>&1 | grep -q "$pattern"; then
        return 0
    fi
    return 1
}

# --- Test 1: Cluster Formation ---
log_info "=================================================="
log_info "TEST 1: Cluster Formation & Discovery"
log_info "=================================================="

log_info "Starting cortex-node1..."
NODE1_ID=$(podman run --name cortex-node1 --network cortex-net -d \
  -v ./test-data/p2p/node1:/data:z \
  --entrypoint sh \
  cortex-hdc -c "touch /tmp/live.log && exec /usr/local/bin/cortex infer --file /tmp/live.log --decay-rate 0.01 --p2p --p2p-bind 7946 --verbose=false")

if [ -z "$NODE1_ID" ]; then
    log_error "Failed to start cortex-node1"
    exit 1
fi

NODE1_PID=$(start_streamer "$NODE1_ID" "test-data/p2p/node1/cortex.log")

# Wait for node1's listener to be up
if wait_for_log "$NODE1_ID" "test-data/p2p/node1/cortex.log" "Starting Inference" 15; then
    log_info "Node 1 is ready."
else
    log_error "Node 1 failed to start within 15 seconds."
    exit 1
fi

log_info "Starting cortex-node2..."
NODE2_ID=$(podman run --name cortex-node2 --network cortex-net -d \
  -v ./test-data/p2p/node2:/data:z \
  --entrypoint sh \
  cortex-hdc -c "touch /tmp/live.log && exec /usr/local/bin/cortex infer --file /tmp/live.log --decay-rate 0.01 --p2p --p2p-bind 7946 --p2p-join cortex-node1:7946 --verbose=false")

if [ -z "$NODE2_ID" ]; then
    log_error "Failed to start cortex-node2"
    exit 1
fi

NODE2_PID=$(start_streamer "$NODE2_ID" "test-data/p2p/node2/cortex.log")

log_info "Starting cortex-node3..."
NODE3_ID=$(podman run --name cortex-node3 --network cortex-net -d \
  -v ./test-data/p2p/node3:/data:z \
  --entrypoint sh \
  cortex-hdc -c "touch /tmp/live.log && exec /usr/local/bin/cortex infer --file /tmp/live.log --decay-rate 0.01 --p2p --p2p-bind 7946 --p2p-join cortex-node1:7946 --verbose=false")

if [ -z "$NODE3_ID" ]; then
    log_error "Failed to start cortex-node3"
    exit 1
fi

NODE3_PID=$(start_streamer "$NODE3_ID" "test-data/p2p/node3/cortex.log")

log_info "Verifying cluster formation..."
# Node 2 and Node 3 should join successfully
if wait_for_log "$NODE2_ID" "test-data/p2p/node2/cortex.log" "Successfully joined cluster" 30; then
    log_info "Node 2 successfully joined the cluster."
else
    log_error "Node 2 failed to join the cluster within 30 seconds."
    exit 1
fi

if wait_for_log "$NODE3_ID" "test-data/p2p/node3/cortex.log" "Successfully joined cluster" 30; then
    log_info "Node 3 successfully joined the cluster."
else
    log_error "Node 3 failed to join the cluster within 30 seconds."
    exit 1
fi

log_success "TEST 1 PASSED: Cluster formed successfully with 3 members."

# --- Test 2: State Replication & Convergence ---
log_info "=================================================="
log_info "TEST 2: State Replication & Convergence"
log_info "=================================================="

# Let's record the start time to measure convergence delay
start_time=$(date +%s)

log_info "Injecting log line into Node 1 (triggering decay update)..."
podman exec "$NODE1_ID" sh -c 'echo "2026-06-22T13:05:33-05:00 INFO [worker] Healthy log line" >> /tmp/live.log'

log_info "Waiting for baseline update replication on Node 2..."
if wait_for_log "$NODE2_ID" "test-data/p2p/node2/cortex.log" "Received baseline update from cluster" 30; then
    end_time_node2=$(date +%s)
    delay_node2=$((end_time_node2 - start_time))
    log_info "Node 2 received replication update in ${delay_node2}s."
else
    log_error "Node 2 did not receive baseline update replication within 30 seconds."
    exit 1
fi

log_info "Waiting for baseline update replication on Node 3..."
if wait_for_log "$NODE3_ID" "test-data/p2p/node3/cortex.log" "Received baseline update from cluster" 30; then
    end_time_node3=$(date +%s)
    delay_node3=$((end_time_node3 - start_time))
    log_info "Node 3 received replication update in ${delay_node3}s."
else
    log_error "Node 3 did not receive baseline update replication within 30 seconds."
    exit 1
fi

log_success "TEST 2 PASSED: Convergence achieved. Propagation delays: Node 2 (${delay_node2}s), Node 3 (${delay_node3}s)."

# --- Test 3: Partition Tolerance / Node Recovery ---
log_info "=================================================="
log_info "TEST 3: Partition Tolerance & Node Recovery"
log_info "=================================================="

log_info "Stopping Node 2 (simulating node crash/network partition)..."
podman stop "$NODE2_ID"
if [ -n "$NODE2_PID" ]; then
    kill "$NODE2_PID" &>/dev/null || true
    NODE2_PID=""
fi

# Verify it stopped
sleep 2

# Check members count on Node 1 (should eventually detect node leaving)
log_info "Injecting new update to Node 1 while Node 2 is offline..."
podman exec "$NODE1_ID" sh -c 'echo "2026-06-22T13:06:00-05:00 INFO [worker] Healthy log line" >> /tmp/live.log'

count_updates() {
    local node_identifier=$1
    local log_file=$2
    local count=0
    if [ -f "$log_file" ]; then
        count=$(grep -c "Received baseline update from cluster" "$log_file" || true)
    fi
    if [ "$count" -eq 0 ] && [ -n "$node_identifier" ]; then
        count=$(podman logs "$node_identifier" 2>&1 | grep -c "Received baseline update from cluster" || true)
    fi
    echo "$count"
}

log_info "Verifying Node 3 received the new update..."
elapsed=0
timeout=30
while [ $elapsed -lt $timeout ]; do
    updates_node3=$(count_updates "$NODE3_ID" "test-data/p2p/node3/cortex.log")
    if [ "$updates_node3" -ge 2 ]; then
        break
    fi
    sleep 1
    elapsed=$((elapsed + 1))
done

if [ "$updates_node3" -ge 2 ]; then
    log_info "Node 3 successfully received update 2 while Node 2 was offline."
else
    log_error "Node 3 failed to receive update 2. Got $updates_node3 updates."
    exit 1
fi

log_info "Restarting Node 2..."
podman start "$NODE2_ID"
NODE2_PID=$(start_streamer "$NODE2_ID" "test-data/p2p/node2/cortex.log")

log_info "Waiting for Node 2 to rejoin the cluster..."
sleep 5

log_info "Triggering new update on Node 1 to test replication to the recovered Node 2..."
start_time_part=$(date +%s)
podman exec "$NODE1_ID" sh -c 'echo "2026-06-22T13:07:00-05:00 INFO [worker] Healthy log line" >> /tmp/live.log'

log_info "Waiting for Node 2 to receive the post-recovery update..."
elapsed=0
timeout=30
while [ $elapsed -lt $timeout ]; do
    updates_node2=$(count_updates "$NODE2_ID" "test-data/p2p/node2/cortex.log")
    if [ "$updates_node2" -ge 2 ]; then
        break
    fi
    sleep 1
    elapsed=$((elapsed + 1))
done

if [ "$updates_node2" -ge 2 ]; then
    end_time_part=$(date +%s)
    delay_part=$((end_time_part - start_time_part))
    log_info "Node 2 successfully received the update after recovery in ${delay_part}s."
else
    log_error "Node 2 failed to receive the update after recovery. Got $updates_node2 updates."
    exit 1
fi

log_success "TEST 3 PASSED: Partition tolerance verified. Recovered node successfully rejoined and replicated state."

# --- Test 4: Node Churn (Dynamic Scaling) ---
log_info "=================================================="
log_info "TEST 4: Node Churn (Dynamic Scaling)"
log_info "=================================================="

log_info "Starting cortex-node4 and joining to Node 1..."
NODE4_ID=$(podman run --name cortex-node4 --network cortex-net -d \
  -v ./test-data/p2p/node4:/data:z \
  --entrypoint sh \
  cortex-hdc -c "touch /tmp/live.log && exec /usr/local/bin/cortex infer --file /tmp/live.log --decay-rate 0.01 --p2p --p2p-bind 7946 --p2p-join cortex-node1:7946 --verbose=false")

if [ -z "$NODE4_ID" ]; then
    log_error "Failed to start cortex-node4"
    exit 1
fi

NODE4_PID=$(start_streamer "$NODE4_ID" "test-data/p2p/node4/cortex.log")

if wait_for_log "$NODE4_ID" "test-data/p2p/node4/cortex.log" "Successfully joined cluster" 30; then
    log_info "Node 4 successfully joined the cluster."
else
    log_error "Node 4 failed to join the cluster within 30 seconds."
    exit 1
fi

log_info "Waiting for cluster state to settle after Node 4 joins..."
sleep 5

log_info "Triggering new update on Node 1..."
start_time_churn=$(date +%s)
podman exec "$NODE1_ID" sh -c 'echo "2026-06-22T13:08:00-05:00 INFO [worker] Healthy log line" >> /tmp/live.log'

log_info "Waiting for Node 4 to receive the update..."
if wait_for_log "$NODE4_ID" "test-data/p2p/node4/cortex.log" "Received baseline update from cluster" 30; then
    end_time_churn=$(date +%s)
    delay_churn=$((end_time_churn - start_time_churn))
    log_info "Node 4 successfully received the update in ${delay_churn}s."
else
    log_error "Node 4 did not receive baseline update replication within 30 seconds."
    exit 1
fi

log_success "TEST 4 PASSED: Node churn verified. Dynamically added node joined and synchronized state successfully."

# --- Test 5: Metrics Extraction ---
log_info "=================================================="
log_info "TEST 5: Metrics Extraction & Resource Utilization"
log_info "=================================================="

log_info "Simulating background load to capture active container metrics..."
for i in {1..5}; do
    podman exec "$NODE1_ID" sh -c "echo '2026-06-22T13:09:0\${i}-05:00 INFO [worker] Healthy log line' >> /tmp/live.log"
    sleep 0.2
done

log_info "Collecting container resource usage..."
echo -e "\n--------------------------------------------------------------------------------"
printf "%-20s %-15s %-25s\n" "CONTAINER NAME" "CPU %" "MEM USAGE / LIMIT"
echo "--------------------------------------------------------------------------------"

# Attempt to get stats using multiple formats for robustness across Podman configurations
podman stats --no-stream cortex-node1 cortex-node2 cortex-node3 cortex-node4 --format "table {{.Name}} \t {{.CPUPerc}} \t {{.MemUsage}}" || \
podman stats --no-stream --format "table {{.Name}} \t {{.CPUPerc}} \t {{.MemUsage}}" | grep cortex-node || \
podman stats --no-stream | grep cortex-node

echo -e "--------------------------------------------------------------------------------\n"

log_success "TEST 5 PASSED: Metrics collected."

log_info "=================================================="
log_success "ALL TESTS PASSED SUCCESSFULLY!"
log_info "=================================================="
