build:
	go build -o bin/vflow ./cmd/vflow

test:
	go test ./...

lint:
	go vet ./...

schema-validate:
	go run ./cmd/vflow schema --validate --format json

doctor:
	go run ./cmd/vflow doctor --format json

doctor-local:
	go run ./cmd/vflow doctor --local --format json

audit:
	go run ./cmd/vflow audit cli --format json
