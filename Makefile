.PHONY: run fmt test test-race vet check

run:
	go run ./cmd/api

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

check: fmt test test-race vet
