GO_FILES := $(shell go list ./... | grep -v examples)
help:	## Display this message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
.PHONY: help
.DEFAULT_GOAL := help

test: ## Run tests
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...
.PHONY: test

lint: ## Run linters
	go vet $(GO_FILES)
	go fmt ./...
	golint -set_exit_status ./...
.PHONY: lint
