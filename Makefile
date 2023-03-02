TEST_ARGS?=
TEST_PACKAGE?=./...

COVERAGE_OUT?=cover.out
COVERAGE_HTML?=coverage.html

COVERAGE_ARGS=-cover -covermode atomic -coverprofile $(COVERAGE_OUT)

.PHONY: test testcover coverage cover

test:
	go test -trimpath -ldflags "-w -s" -race $(TEST_ARGS) $(TEST_PACKAGE)

testcover:
	go test -trimpath -ldflags "-w -s" -race $(COVERAGE_ARGS) $(TEST_ARGS) $(TEST_PACKAGE)

coverage:
	go tool cover -html $(COVERAGE_OUT) -o $(COVERAGE_HTML)

cover: testcover coverage

.PHONY: fmt vet prepare

fmt:
	goimports -w .

vet:
	go vet ./...

prepare: fmt vet
