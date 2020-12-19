SHELL = bash
GO_FILES = $(shell find . -name "*.go" | grep -vE ".git")
GO_COVER_FILE = `find . -name "coverage.out"`

.PHONY: all test format cover-clean check fmt vet lint

test: $(GO_FILES)
	go test ./...

format:
	gofmt -s -w ${GO_FILES}

cover: $(GO_FILES)
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

cover-clean:
	rm -f $(GO_COVER_FILE)

deps:
	go mod download

check:
	if [ -n "$(shell gofmt -l ${GO_FILES})" ]; then \
		echo 1>&2 'The following files need to be formatted:'; \
		gofmt -l .; \
		exit 1; \
	fi

vet:
	go vet $(GO_FILES)

lint:
	golint $(GO_FILES)
