## PROLOG

.PHONY: help all

CMDNAME=kerrors
CMDDESC=errors with context

help: ## Print this help
	@./help.sh '$(CMDNAME)' '$(CMDDESC)'

all: test ## Default

## TESTS

TEST_ARGS?=
COVERAGE?=cover.out

.PHONY: test coverage cover bench

test: ## Run tests
	go test -race -trimpath -ldflags "-w -s" $(TEST_ARGS) -cover -covermode atomic -coverprofile $(COVERAGE) ./...

coverage: ## View test coverage
	go tool cover -html $(COVERAGE)

cover: test coverage ## Create coverage report

## FMT

.PHONY: fmt vet prepare

fmt: ## Run go fmt
	goimports -w .

vet: ## Lint code
	go vet ./...

prepare: fmt vet ## Prepare code for PR
