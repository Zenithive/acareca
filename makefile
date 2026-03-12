dev:
	go run ./cmd/api

vet:
	go vet ./...

build:
	go build -o acareca ./cmd/api
