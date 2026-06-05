.PHONY: build build-api test infra-up infra-down infra-clean clean run-api

build: build-api
	go build -o bin/worker cmd/worker/main.go

build-api:
	go build -o bin/api cmd/api/main.go

test:
	@# Run tests with PostgreSQL integration tests if available
	@if [ -n "$$POSTGRES_HOST" ]; then \
		echo "Running tests WITH PostgreSQL integration..."; \
		go test -v -race -count=1 -coverprofile=coverage.out ./...; \
	else \
		echo "Running tests without PostgreSQL (set POSTGRES_HOST to enable integration tests)..."; \
		go test -v -race -count=1 ./...; \
	fi

test-cover: test
	go tool cover -func=coverage.out
	@echo ""
	@go tool cover -func=coverage.out | tail -1

run-api:
	go run ./cmd/api

infra-up:
	podman rm -f clickhouse-test qdrant-test minio-test rabbitmq-test postgres-test || true
	podman run -d --name clickhouse-test -p 9000:9000 -p 8123:8123 -e CLICKHOUSE_USER=default -e CLICKHOUSE_PASSWORD=test -e CLICKHOUSE_DB=magicstrike -v clickhouse_data:/var/lib/clickhouse docker.io/clickhouse/clickhouse-server:latest
	podman run -d --name qdrant-test -p 6333:6333 -p 6334:6334 -v qdrant_data:/qdrant/storage docker.io/qdrant/qdrant:latest
	podman run -d --name minio-test -p 9002:9000 -p 9001:9001 -e MINIO_ROOT_USER=minioadmin -e MINIO_ROOT_PASSWORD=minioadmin -v minio_data:/data docker.io/minio/minio:latest server /data --console-address ":9001"
	podman run -d --name rabbitmq-test -p 5672:5672 -p 15672:15672 -e RABBITMQ_DEFAULT_USER=guest -e RABBITMQ_DEFAULT_PASS=guest docker.io/rabbitmq:3-management
	podman run -d --name postgres-test -p 5432:5432 -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=magicstrike -v postgres_data:/var/lib/postgresql/data docker.io/postgres:16-alpine
	@echo "Infrastructure started with persistent volumes (clickhouse_data, qdrant_data, minio_data, postgres_data):"
	@echo "  ClickHouse:  localhost:9000 (native), localhost:8123 (HTTP)"
	@echo "  Qdrant:      localhost:6333 (HTTP), localhost:6334 (gRPC)"
	@echo "  Minio:       localhost:9002 (API), localhost:9001 (Console)"
	@echo "  RabbitMQ:    localhost:5672 (AMQP), localhost:15672 (Management)"
	@echo "  Postgres:    localhost:5432"
	@echo ""
	@echo "Wait a few seconds for all services to be ready..."

infra-down:
	podman rm -f clickhouse-test qdrant-test minio-test rabbitmq-test postgres-test || true

infra-clean: infra-down
	podman volume rm -f clickhouse_data qdrant_data minio_data postgres_data || true

clean:
	rm -rf bin/
