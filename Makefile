.PHONY: build run test lint migrate docker-up docker-down tidy swag \
        build-prod deploy deploy-rollback backup restore \
        docker-prod-up docker-prod-down docker-prod-logs \
        gen-secrets health-check

APP_NAME   = vmorbit
BUILD_DIR  = ./bin
CMD_PATH   = ./cmd/server

## build: Compile the binary (development)
build:
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(APP_NAME) $(CMD_PATH)

## build-prod: Compile the binary with version info (production)
build-prod:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	go build \
	  -ldflags="-s -w -X main.version=$$(git describe --tags --always 2>/dev/null || echo dev)" \
	  -trimpath \
	  -o $(BUILD_DIR)/$(APP_NAME) $(CMD_PATH)

## run: Run the server locally
run:
	go run $(CMD_PATH)/main.go

## test: Run all tests
test:
	go test ./... -v -race -count=1

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## tidy: Tidy and verify go modules
tidy:
	go mod tidy
	go mod verify

## swag: Generate Swagger docs
swag:
	swag init -g $(CMD_PATH)/main.go -o ./docs

## docker-up: Start Postgres + Redis via Docker Compose (development)
docker-up:
	docker compose up -d postgres redis

## docker-down: Stop all Docker Compose services (development)
docker-down:
	docker compose down

## migrate: Run database migrations (uses GORM AutoMigrate via the app)
migrate:
	go run ./cmd/migrate/main.go

## ── Production targets ────────────────────────────────────────────────────────

## docker-prod-up: Start all production services
docker-prod-up:
	docker compose -f docker-compose.production.yml --env-file .env.production up -d

## docker-prod-down: Stop all production services
docker-prod-down:
	docker compose -f docker-compose.production.yml down

## docker-prod-logs: Tail production logs
docker-prod-logs:
	docker compose -f docker-compose.production.yml logs -f --tail=100

## deploy: Full production deployment (build + deploy)
deploy:
	./scripts/deploy.sh

## deploy-rollback: Roll back to the previous version
deploy-rollback:
	./scripts/deploy.sh --rollback

## backup: Run a manual database backup
backup:
	./scripts/backup.sh

## restore: Restore from a backup file (BACKUP_FILE=./backups/vmorbit_xxx.sql.gz)
restore:
	./scripts/backup.sh --restore $(BACKUP_FILE)

## gen-secrets: Generate production secrets (prints to stdout — save to .env.production)
gen-secrets:
	@echo "# Generated secrets for .env.production"
	@echo "VMORBIT_JWT_SECRET=$$(openssl rand -base64 64 | tr -d '\n')"
	@echo "VMORBIT_ENCRYPTION_KEY=$$(openssl rand -hex 32)"
	@echo "DB_PASSWORD=$$(openssl rand -base64 32 | tr -d '\n/+=' | head -c 32)"
	@echo "REDIS_PASSWORD=$$(openssl rand -base64 32 | tr -d '\n/+=' | head -c 32)"

## health-check: Check production service health
health-check:
	@echo "=== Liveness ===" && curl -sf http://localhost/health | python3 -m json.tool
	@echo "=== Readiness ===" && curl -sf http://localhost/ready | python3 -m json.tool
	@echo "=== Status ===" && curl -sf http://localhost/status | python3 -m json.tool
