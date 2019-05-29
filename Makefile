help:	## Display this message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
.PHONY: help
.DEFAULT_GOAL := help

test: ## Run tests
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...
.PHONY: test

lint: ## Run linters
	go vet ./...
	test -z $(shell gofmt -l ./..) || (gofmt -l -d ./..; exit 1)
	go run golang.org/x/lint/golint -set-exit-status ./...
.PHONY: lint
