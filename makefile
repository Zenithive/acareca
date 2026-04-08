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

seed:
	go run scripts/seed_clinics_forms.go

seed-advanced:
	go run scripts/seed_advanced.go

seed-large:
	go run scripts/seed_advanced.go -clinics 50 -forms 10 -verbose

seed-cleanup:
	go run scripts/cleanup_seed.go -confirm

seed-cleanup-dry:
	go run scripts/cleanup_seed.go -dry-run

cleanup-practitioner:
	@echo "Usage: make cleanup-practitioner PRACTITIONER_ID=<uuid>"
	@if [ -z "$(PRACTITIONER_ID)" ]; then \
		echo "Error: PRACTITIONER_ID is required"; \
		echo "Example: make cleanup-practitioner PRACTITIONER_ID=c9b2ecb5-ced4-43ea-9507-ac9ba56482f0"; \
		exit 1; \
	fi
	go run scripts/cleanup_practitioner.go -practitioner-id $(PRACTITIONER_ID) -confirm

cleanup-practitioner-dry:
	@echo "Usage: make cleanup-practitioner-dry PRACTITIONER_ID=<uuid>"
	@if [ -z "$(PRACTITIONER_ID)" ]; then \
		echo "Error: PRACTITIONER_ID is required"; \
		echo "Example: make cleanup-practitioner-dry PRACTITIONER_ID=c9b2ecb5-ced4-43ea-9507-ac9ba56482f0"; \
		exit 1; \
	fi
	go run scripts/cleanup_practitioner.go -practitioner-id $(PRACTITIONER_ID) -dry-run