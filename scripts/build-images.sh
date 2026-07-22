#!/bin/bash
set -e

echo "Building hivemind-scoring (Python)..."
docker build -f services/scoring-python/Dockerfile -t hivemind-scoring:latest .

echo "Building hivemind-worker (Go)..."
docker build -f Dockerfile.go --build-arg SERVICE_PATH=./cmd/worker -t hivemind-worker:latest .

echo "Building hivemind-dispatcher (Go)..."
docker build -f Dockerfile.go --build-arg SERVICE_PATH=./cmd/dispatcher -t hivemind-dispatcher:latest .

echo "Building hivemind-heartbeat-reaper (Go)..."
docker build -f Dockerfile.go --build-arg SERVICE_PATH=./cmd/heartbeat-reaper -t hivemind-heartbeat-reaper:latest .

echo "Building hivemind-salience-decay (Go)..."
docker build -f Dockerfile.go --build-arg SERVICE_PATH=./cmd/salience-decay -t hivemind-salience-decay:latest .

echo "Building hivemind-scoring-api (Go)..."
docker build -f Dockerfile.go --build-arg SERVICE_PATH=./cmd/scoring-api -t hivemind-scoring-api:latest .

echo "Building hivemind-review-api (Go)..."
docker build -f Dockerfile.go --build-arg SERVICE_PATH=./cmd/review-api -t hivemind-review-api:latest .

echo "All images built successfully."
docker images | grep hivemind
