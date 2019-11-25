GO_FILES := $(wildcard *.go)

monitoring: $(GO_FILES)
	go build -a -tags netgo -ldflags '-w -extldflags "-static"' .
	docker build -t 'application:latest' .

build: monitoring ## Builds app binary
.PHONY: build

run: monitoring ## Runs application inside docker-compose (sync)
	docker-compose up
.PHONY: run

start: monitoring ## Spawns application inside docker-compose (async)
	docker-compose up -d
.PHONY: start

help:	## Display this message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
.PHONY: help
.DEFAULT_GOAL := help
