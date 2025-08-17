# Indonesian Regions Fuzzy Search API

A high-performance, dependency-free Go API for fuzzy searching Indonesian administrative regions using DuckDB. This service provides fast and accurate search capabilities for Indonesian provinces, cities, districts, and subdistricts.

## Table of Contents

- [Features](#features)
- [API Usage](#api-usage)
  - [Search Endpoint](#search-endpoint)
  - [Health Check Endpoint](#health-check-endpoint)
- [Configuration](#configuration)
- [Quick Start](#quick-start)
  - [Prerequisites](#prerequisites)
  - [Using Makefile](#using-makefile)
  - [Manual Build and Run](#manual-build-and-run)
  - [Using Docker](#using-docker)
- [Deployment](#deployment)
  - [Building and Pushing Docker Image](#building-and-pushing-docker-image)
  - [Deploying to Cloud Providers](#deploying-to-cloud-providers)
- [Maintenance](#maintenance)
  - [Updating Administrative Data](#updating-administrative-data)
- [Makefile Commands](#makefile-commands)
- [Acknowledgements](#acknowledgements)
- [Project Structure](#project-structure)

## Features

- **Fuzzy Search**: Uses Levenshtein distance algorithm for typo-tolerant searches
- **High Performance**: Powered by DuckDB for fast querying of Indonesian administrative data
- **Lightweight**: Minimal dependencies with GoFiber web framework
- **Container Ready**: Dockerized application for easy deployment
- **Configurable**: Environment-based configuration for port and database path

## API Usage

### Search Endpoint

```
GET /v1/search?q={query}
```

**Parameters:**
- `q` (required): Search query string (e.g., "bandung")

**Example Request:**
```bash
curl "http://localhost:8080/v1/search?q=bandung"
```

**Example Response:**
```json
[
  {
    "id": "3273010001",
    "subdistrict": "Sukasari",
    "district": "Sukasari",
    "city": "Kota Bandung",
    "province": "Jawa Barat",
    "full_text": "jawa barat kota bandung sukasari sukasari"
  },
  {
    "id": "3273020001",
    "subdistrict": "Cidadap",
    "district": "Cidadap",
    "city": "Kota Bandung",
    "province": "Jawa Barat",
    "full_text": "jawa barat kota bandung cidadap cidadap"
  }
]
```

### Specific Search Endpoints

In addition to the general search endpoint, the API provides specific search endpoints for each administrative level:

- **District Search**: `/v1/search/district?q={query}`
- **Subdistrict Search**: `/v1/search/subdistrict?q={query}`
- **City Search**: `/v1/search/city?q={query}`
- **Province Search**: `/v1/search/province?q={query}`

Each specific search endpoint:
- Takes a required `q` query parameter containing the search term
- Returns a JSON array of matching regions at that administrative level
- Uses the Jaro-Winkler similarity algorithm for fuzzy matching with a threshold of 0.8
- Limits results to 10 items
- Returns the same Region structure as the general search endpoint

#### District Search Endpoint

```
GET /v1/search/district?q={query}
```

**Parameters:**
- `q` (required): Search query string (e.g., "bandung")

**Example Request:**
```bash
curl "http://localhost:8080/v1/search/district?q=bandung"
```

#### Subdistrict Search Endpoint

```
GET /v1/search/subdistrict?q={query}
```

**Parameters:**
- `q` (required): Search query string (e.g., "sukasari")

**Example Request:**
```bash
curl "http://localhost:8080/v1/search/subdistrict?q=sukasari"
```

#### City Search Endpoint

```
GET /v1/search/city?q={query}
```

**Parameters:**
- `q` (required): Search query string (e.g., "bandung")

**Example Request:**
```bash
curl "http://localhost:8080/v1/search/city?q=bandung"
```

#### Province Search Endpoint

```
GET /v1/search/province?q={query}
```

**Parameters:**
- `q` (required): Search query string (e.g., "jawa")

**Example Request:**
```bash
curl "http://localhost:8080/v1/search/province?q=jawa"
```

### Health Check Endpoint

```
GET /healthz
```

**Example Request:**
```bash
curl "http://localhost:8080/healthz"
```

**Example Response:**
```json
{
  "status": "ok",
  "message": "Service is healthy"
}
```

## Configuration

The application can be configured using the following environment variables:

| Variable | Description | Default Value |
|----------|-------------|---------------|
| `PORT` | Port for the API server to listen on | `8080` |
| `DB_PATH` | Path to the DuckDB database file | `data/regions.duckdb` |

## Quick Start

### Prerequisites

- Go 1.21 or higher
- curl (for downloading data)
- Docker (optional, for containerized deployment)

### Using Makefile

The easiest way to get started is by using the provided Makefile:

```bash
# Download the administrative data and prepare the database
make prepare-db

# Run the API server
make run
```

### Manual Build and Run

1. **Download the data:**
   ```bash
   curl -o data/wilayah.sql https://raw.githubusercontent.com/cahyadsn/wilayah/master/db/wilayah.sql
   ```

2. **Prepare the database:**
   ```bash
   go run ./cmd/ingestor/main.go
   ```

3. **Run the API server:**
   ```bash
   go run ./cmd/api/main.go
   ```

### Using Docker

1. **Build the Docker image:**
   ```bash
   docker build -t regions-api .
   ```

2. **Run the container:**
   ```bash
   docker run -p 8080:8080 regions-api
   ```

## Deployment

### Building and Pushing Docker Image

To build and push the Docker image to a container registry:

```bash
# Build the image
docker build -t your-registry/regions-api:latest .

# Push to container registry
docker push your-registry/regions-api:latest
```

### Deploying to Cloud Providers

#### Fly.io

1. Install the Fly.io CLI
2. Create a fly.toml file:
   ```toml
   app = "regions-api"
   
   [build]
     dockerfile = "Dockerfile"
   
   [env]
     PORT = "8080"
   
   [[services]]
     internal_port = 8080
     protocol = "tcp"
   
     [[services.ports]]
       port = 80
       handlers = ["http"]
   
     [[services.ports]]
       port = 443
       handlers = ["tls", "http"]
   ```

3. Deploy:
   ```bash
   flyctl launch
   ```

#### Railway

1. Connect your GitHub repository to Railway
2. Set environment variables in Railway dashboard:
   - PORT: 8080
3. Railway will automatically build and deploy using the Dockerfile

#### DigitalOcean App Platform

1. Create a new app and connect your repository
2. Set environment variables:
   - PORT: 8080
3. Set the build command to:
   ```bash
   docker build -t regions-api .
   ```
4. Set the run command to:
   ```bash
   docker run -p $PORT:8080 regions-api
   ```

## Maintenance

### Updating Administrative Data

To update the regions database with new administrative data:

1. **Download the latest data:**
   ```bash
   make download-data
   ```

2. **Reprocess the data:**
   ```bash
   make ingest
   ```

   Or run the ingestor manually:
   ```bash
   go run ./cmd/ingestor/main.go
   ```

This process will:
- Download the latest `wilayah.sql` file
- Create a new `regions.duckdb` database
- Transform the hierarchical data into a denormalized table for efficient searching
- Clean up temporary tables to keep the database file small

## Makefile Commands

| Command | Description |
|---------|-------------|
| `make prepare-db` | Download data and run ingestor (recommended for first run) |
| `make run` | Run the API server |
| `make ingest` | Run the data ingestor |
| `make download-data` | Download the SQL data file |
| `make build` | Build the API binary |
| `make docker-build` | Build Docker image |
| `make docker-run` | Run Docker container |
| `make test` | Run tests |
| `make clean` | Clean build artifacts |
| `make deps` | Install dependencies |
| `make help` | Show help message |

## Acknowledgements

We would like to express our gratitude to [cahyadsn](https://github.com/cahyadsn) for contributing the Indonesian administrative regions data that powers this API. The data is sourced from the [wilayah](https://github.com/cahyadsn/wilayah) repository, which provides comprehensive and up-to-date information about Indonesian provinces, cities, districts, and subdistricts.

## Project Structure

```
.
├── cmd/
│   ├── api/          # Main application entrypoint
│   └── ingestor/     # Data ingestion script
├── data/
│   ├── regions.duckdb # DuckDB database file (generated)
│   └── wilayah.sql   # Raw SQL data file (downloaded)
├── internal/
│   └── api/          # API handlers and routing
├── Dockerfile        # Docker configuration
├── Makefile          # Build and run commands
├── go.mod            # Go module file
└── go.sum            # Go checksum file