dev:
	go run ./cmd/api

build:
	go build -o acareca ./cmd/api

swagger:
	swag init -g cmd/api/main.go

postman: docs/swagger.json
	@openapi2postmanv2 -s docs/swagger.json -o postman/collection.json

docs/swagger.json: swagger
	@:

vet:
	go vet ./...