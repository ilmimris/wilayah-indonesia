# Makefile for Indonesian Regions Fuzzy Search API

# Variables
BINARY=regions-api
MAIN_DIR=cmd/api
INGESTOR_DIR=cmd/ingestor
DATA_DIR=data
DB_FILE=$(DATA_DIR)/regions.duckdb
SQL_FILE=$(DATA_DIR)/wilayah.sql
KODEPOS_FILE=$(DATA_DIR)/wilayah_kodepos.sql

# Default target
.PHONY: all
all: build

# Build the API binary
.PHONY: build
build:
	go build -o $(BINARY) ./$(MAIN_DIR)

# Run the API server
.PHONY: run
run:
	go run ./$(MAIN_DIR)

# Run the data ingestor
.PHONY: ingest
ingest:
	go run ./$(INGESTOR_DIR)
# Download the administrative data SQL file
.PHONY: download-data
download-data: download-admin-data download-kodepos-data

# Download the administrative data SQL file
.PHONY: download-admin-data
download-admin-data:
	curl -o $(SQL_FILE) https://raw.githubusercontent.com/cahyadsn/wilayah/master/db/wilayah.sql

# Download the postal code data SQL file
.PHONY: download-kodepos-data
download-kodepos-data:
	curl -o $(KODEPOS_FILE) https://raw.githubusercontent.com/cahyadsn/wilayah_kodepos/refs/heads/main/db/wilayah_kodepos.sql

# Prepare the database (download data and run ingestor)
.PHONY: prepare-db
prepare-db: download-data ingest


# Run tests
.PHONY: test
test:
	go test -v ./...
# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY)
	rm -f $(DB_FILE)
	rm -f $(SQL_FILE)
	rm -f $(KODEPOS_FILE)


# Install dependencies
.PHONY: deps
deps:
	go mod tidy

# Build Docker image
.PHONY: docker-build
docker-build:
	docker build -t $(BINARY) .

# Run Docker container
.PHONY: docker-run
docker-run:
	docker run -p 8080:8080 $(BINARY)

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all          - Build the API binary (default)"
	@echo "  build        - Build the API binary"
	@echo "  run          - Run the API server"
	@echo "  ingest       - Run the data ingestor"
	@echo "  download-data - Download all data files"
	@echo "  download-admin-data - Download administrative data file"
	@echo "  download-kodepos-data - Download postal code data file"
	@echo "  prepare-db   - Download data and run ingestor"
	@echo "  test         - Run tests"
	@echo "  clean        - Clean build artifacts and data files"
	@echo "  deps         - Install dependencies"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run Docker container"
	@echo "  help         - Show this help message"