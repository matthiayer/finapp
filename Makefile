.PHONY: build test test-browser vet check run

build:
	mkdir -p build
	go build -trimpath -o build/finapp ./cmd/finapp

test:
	go test ./...

test-browser:
	npm run test:browser

vet:
	go vet ./...

check: test vet build

run:
	go run ./cmd/finapp
