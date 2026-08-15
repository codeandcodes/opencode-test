.PHONY: build test run ui check

build:
	go build -o opencode-bench .

test:
	go test ./...

run: build
	./opencode-bench

ui:
	cd ui && npm run build

check:
	go vet ./...
	go test ./...
