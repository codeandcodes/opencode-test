.PHONY: build test run ui check

build: ui
	go build -tags ui -o opencode-bench .

test:
	go test ./...

run: build
	./opencode-bench

ui:
	cd ui && npm run build
	rm -rf internal/server/ui_dist
	cp -r ui/dist internal/server/ui_dist

check:
	go vet ./...
	go test ./...
	cd ui && npx vitest run && npm run build
