.PHONY: all build train infer test clean

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
