.PHONY: all build train infer test clean proto proto-deps

export PATH := $(shell go env GOPATH)/bin:$(PATH)

APP_NAME = cortex
CMD_DIR = ./cmd/cortex
DB_FILE = cortex.kv

all: build

build:
	go build -o $(APP_NAME) $(CMD_DIR)

train: build
	./$(APP_NAME) train --file train.log

infer: build
	./$(APP_NAME) infer --file live.log

test:
	go test -v ./...

clean:
	rm -f $(APP_NAME) $(DB_FILE)

proto-deps:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/proto/v1/telemetry.proto

