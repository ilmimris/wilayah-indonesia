# Gemini Code Assistant Context

This document provides context for the Gemini code assistant to understand the project structure, technologies, and conventions.

## Project Overview

This project is a high-performance, dependency-free Go API for fuzzy searching Indonesian administrative regions. It uses [DuckDB](https://duckdb.org/) for fast, in-process analytical queries and the [GoFiber](https://gofiber.io/) web framework for lightweight and efficient routing.

The core functionality revolves around searching for Indonesian provinces, cities, districts, and sub-districts using typo-tolerant fuzzy search powered by the Jaro-Winkler similarity algorithm.

The application is designed to be run as a standalone binary or as a Docker container.

### Key Technologies

*   **Language:** Go (version 1.24)
*   **Web Framework:** GoFiber
*   **Database:** DuckDB (embedded)
*   **Containerization:** Docker

### Architecture

The project follows a standard Go project layout:

*   `cmd/`: Contains the main application entry points.
    *   `api/`: The main entry point for the web API.
    *   `ingestor/`: A script to ingest and process the administrative data into the DuckDB database.
*   `internal/`: Houses the internal application logic, primarily the API handlers.
*   `pkg/`: Contains the core service logic, which is reusable and decoupled from the API layer.
*   `data/`: Stores the DuckDB database file.
*   `helm-chart/`: Contains a Helm chart for Kubernetes deployment.

## Building and Running

The project uses a `Makefile` to streamline common development tasks.

### Initial Setup

To get the project running for the first time, you need to prepare the database. This command downloads the necessary SQL data files and runs the ingestor script to create the `regions.duckdb` file.

```bash
make prepare-db
```

### Running the API

To run the API server locally:

```bash
make run
```

The server will start on port `8080` by default.

### Running Tests

To run the test suite:

```bash
make test
```

### Building the Binary

To build the application into a single binary:

```bash
make build
```

### Docker

To build and run the application using Docker:

```bash
# Build the image
make docker-build

# Run the container
make docker-run
```

## Development Conventions

*   **Configuration:** Application configuration is managed through environment variables. The key variables are:
    *   `PORT`: The port for the API server (default: `8080`).
    *   `DB_PATH`: The path to the DuckDB database file (default: `data/regions.duckdb`).
*   **Logging:** The project uses the standard `log/slog` package for structured logging.
*   **Dependencies:** Go modules are used for dependency management. Use `go mod tidy` to keep dependencies clean.
*   **API:** The API is versioned under `/v1`. See `api.rest` for example requests.
*   **Error Handling:** Custom error types are defined in `pkg/service/errors.go` to provide more context on failures.
