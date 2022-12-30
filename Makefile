.PHONY: help test testall

help: ## Show help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

test: ## Run unit tests
	go test -v -mod=vendor -short -coverprofile=coverage.txt -covermode=atomic ./...

testall: ## Run all tests
	docker-compose up -d
	sleep 1
	go test -v -mod=vendor -coverprofile=coverage.txt -covermode=atomic ./...
	docker-compose down -v

