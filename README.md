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

- **BM25 Full-Text Search**: Utilizes DuckDB's `match_bm25` for fast and relevant full-text search across all administrative levels.
- **Fuzzy Search**: Employs the Jaro-Winkler similarity algorithm for typo-tolerant searches on specific administrative levels (province, city, district, subdistrict).
- **BPS Integration**: Optionally search and respond with official BPS (Badan Pusat Statistik) codes and names.
- **High Performance**: Powered by DuckDB for fast querying of Indonesian administrative data.
- **Lightweight**: Minimal dependencies with the GoFiber web framework.
- **Container Ready**: Dockerized application for easy deployment.
- **Clean Architecture**: Delivery, use case, repository, and gateway layers are isolated to keep business rules portable.
- **Configurable**: Environment-based configuration for port, database path, and ingestion data directory.

## Architecture Overview

The codebase follows a Clean Architecture layout:

- `cmd/api`, `cmd/ingestor` – binary entrypoints that delegate to internal bootstrappers.
- `internal/config` – central wiring for loggers, DuckDB connections, Fiber apps, and use cases.
- `internal/delivery/http` – Fiber controllers, routes, and middleware for the public API.
- `internal/delivery/worker` – CLI-facing delivery adapter that runs dataset refresh workflows.
- `internal/usecase` – business rules for region search and dataset ingestion.
- `internal/repository/duckdb` – data-access implementations and administrative helpers for DuckDB.
- `internal/gateway` – filesystem loader and SQL normalizer abstractions used by the ingestion flow.
- `internal/shared` – cross-cutting concerns such as error taxonomy.

## API Usage

### Search Endpoint

The general search endpoint supports both full-text and field-level fuzzy filters. Use any combination of the parameters below to narrow results.

- **BM25 Full-Text Search**: `q` uses DuckDB `match_bm25` over a combined `full_text` column.
- **Field Fuzzy Filters**: `subdistrict`, `district`, `city`, `province` use Jaro-Winkler (≥ 0.8).
- **Composable**: Combine parameters; scores are aggregated and results ordered by total score.
- **City matching**: `city` matches both "Kota {name}" and "Kabupaten {name}" automatically.
- **Performance**: Returns top 10 results.

```
GET /v1/search?q={query}&subdistrict={name}&district={name}&city={name}&province={name}
```

**Parameters:**
- `q` (optional): Full-text query (e.g., "bandung").
- `subdistrict` (optional): Fuzzy filter for subdistrict.
- `district` (optional): Fuzzy filter for district.
- `city` (optional): Fuzzy filter for city/regency (no need to prefix with Kota/Kabupaten).
- `province` (optional): Fuzzy filter for province.
- `limit` (optional): Maximum records to return (defaults to 10, capped at 100).
- `search_bps` (optional): When `true`, fuzzy comparisons use BPS (Badan Pusat Statistik) names.
- `include_bps` (optional): When `true`, the response adds BPS codes and names for each level.
- `include_scores` (optional): When `true`, the response adds the full-text score and per-field similarity scores.

**Example Requests:**
```bash
# Full-text only
curl "http://localhost:8080/v1/search?q=bandung"

# Combine full-text with province filter
curl "http://localhost:8080/v1/search?q=bandung&province=Jawa Barat"

# Field-only filters (no q)
curl "http://localhost:8080/v1/search?district=Cidadap&city=Bandung&province=Jawa Barat"

# Request BPS metadata and scoring
curl "http://localhost:8080/v1/search?q=bandung&include_bps=true&include_scores=true&limit=5"

# Search using BPS naming
curl "http://localhost:8080/v1/search?q=kemayoran&search_bps=true&include_bps=true"
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
    "postal_code": "40151",
    "full_text": "40151 jawa barat kota bandung sukasari sukasari",
    "bps": {
      "subdistrict": {"code": "3273010001", "name": "Sukasari"},
      "district": {"code": "3273010", "name": "Sukasari"},
      "city": {"code": "3273", "name": "Bandung"},
      "province": {"code": "32", "name": "Jawa Barat"}
    },
    "scores": {
      "fts": 6.82,
      "subdistrict": 0.96,
      "district": 0.94,
      "city": 0.91,
      "province": 0.88
    }
  }
]
```

### Specific Search Endpoints

In addition to the general search endpoint, the API provides specific search endpoints for each administrative level. All use Jaro-Winkler fuzzy matching (≥ 0.8), order by similarity, and return up to 10 results by default. They also honour the shared `limit`, `include_bps`, and `include_scores` toggles.

- **District Search**: `/v1/search/district?q={district}&city={city}&province={province}&limit={n}&include_bps={bool}&include_scores={bool}`
  - `q` is required; `city` and `province` are optional narrowing filters.
  - `city` matches both Kota and Kabupaten prefixes automatically.

- **Subdistrict Search**: `/v1/search/subdistrict?q={subdistrict}&district={district}&city={city}&province={province}&limit={n}&include_bps={bool}&include_scores={bool}`
  - `q` is required; `district`, `city`, and `province` are optional narrowing filters.
  - `city` matches both Kota and Kabupaten prefixes automatically.

- **City Search**: `/v1/search/city?q={city}&limit={n}&include_bps={bool}&include_scores={bool}`
  - `q` is required; matches both Kota and Kabupaten.

- **Province Search**: `/v1/search/province?q={province}&limit={n}&include_bps={bool}&include_scores={bool}`
  - `q` is required.

#### District Search Endpoint

```
GET /v1/search/district?q={district}&city={city}&province={province}&limit={n}&include_bps={bool}&include_scores={bool}
```

**Parameters:**
- `q` (required): District name to match (e.g., "sukasari").
- `city` (optional): Narrow by city/regency (no need for Kota/Kabupaten).
- `province` (optional): Narrow by province.

**Example Requests:**
```bash
curl "http://localhost:8080/v1/search/district?q=Cidadap"
curl "http://localhost:8080/v1/search/district?q=Cidadap&city=Bandung&province=Jawa Barat"
```

#### Subdistrict Search Endpoint

```
GET /v1/search/subdistrict?q={subdistrict}&district={district}&city={city}&province={province}&limit={n}&include_bps={bool}&include_scores={bool}
```

**Parameters:**
- `q` (required): Subdistrict name (e.g., "sukasari").
- `district` (optional): Narrow by district.
- `city` (optional): Narrow by city/regency (Kota/Kabupaten handled automatically).
- `province` (optional): Narrow by province.

**Example Requests:**
```bash
curl "http://localhost:8080/v1/search/subdistrict?q=Sukasari"
curl "http://localhost:8080/v1/search/subdistrict?q=Sukasari&district=Sukasari&city=Bandung&province=Jawa Barat"
```

#### City Search Endpoint

```
GET /v1/search/city?q={query}&limit={n}&include_bps={bool}&include_scores={bool}
```

**Parameters:**
- `q` (required): Search query string (e.g., "bandung")

**Example Request:**
```bash
curl "http://localhost:8080/v1/search/city?q=bandung"
```

#### Province Search Endpoint

```
GET /v1/search/province?q={query}&limit={n}&include_bps={bool}&include_scores={bool}
```

**Parameters:**
- `q` (required): Search query string (e.g., "jawa")

**Example Request:**
```bash
curl "http://localhost:8080/v1/search/province?q=jawa"
```
#### Postal Code Search Endpoint

```
GET /v1/search/postal/{postalCode}?limit={n}&include_bps={bool}&include_scores={bool}
```

**Parameters:**
- `postalCode` (required): 5-digit postal code (e.g., "10110")

**Example Request:**
```bash
curl "http://localhost:8080/v1/search/postal/10110"
```

**Example Response:**
```json
[
  {
    "id": "3101010001",
    "subdistrict": "Kepulauan Seribu Utara",
    "district": "Kepulauan Seribu Utara",
    "city": "Kabupaten Kepulauan Seribu",
    "province": "DKI Jakarta",
    "postal_code": "10110",
    "full_text": "dki jakarta kabupaten kepulauan seribu kepulauan seribu utara kepulauan seribu utara"
  }
]
```

The postal code search endpoint:
- Takes a required `postalCode` path parameter containing a 5-digit postal code
- Returns a JSON array of matching regions with that postal code
- Performs an exact match on the postal code
- Limits results to 10 items
- Returns the same Region structure as other search endpoints
- Returns a 404 error if no regions are found for the provided postal code
- Returns a 400 error if the postal code is not a valid 5-digit number

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
| `DB_PATH` | Path to the DuckDB database file. The API opens it read-only; the ingestor opens it read-write. | `data/regions.duckdb` |
| `DATA_DIR` | Base directory containing SQL dumps used by the ingestor (`wilayah.sql`, `wilayah_kodepos.sql`, `bps_wilayah.sql`) | `data/` |
| `MATCHER_SNAPSHOT_PATH` | Location of the matcher snapshot JSON used to hydrate fuzzy indexes | _required_ |
| `MATCHER_MIN_SCORE` | Minimum combined suggestion score required before auto-filling hierarchical filters | `0.8` |
| `MATCHER_THRESHOLD_PROVINCE` | Province-level similarity threshold applied to matcher suggestions | `0.4` |
| `MATCHER_THRESHOLD_CITY` | City/regency similarity threshold applied to matcher suggestions | `0.5` |
| `MATCHER_THRESHOLD_DISTRICT` | District similarity threshold applied to matcher suggestions | `0.58` |
| `MATCHER_THRESHOLD_SUBDISTRICT` | Subdistrict similarity threshold applied to matcher suggestions | `0.45` |
| `MATCHER_WORD_COMBO_SIZE` | Maximum contiguous word combination length used when generating fragments | `3` |
| `MATCHER_PARALLEL_TOP_K` | Number of matches retained per administrative level during parallel fragment searches | `50` |

### Matcher tuning

The matcher consumes a JSON snapshot (generated by the ingestor) to hydrate fuzzy indexes. By default it:

- Requires a combined suggestion score of **0.8** before automatically populating request filters.
- Enforces similarity thresholds of **0.4** (province), **0.5** (city/regency), **0.58** (district), and **0.45** (subdistrict).
- Generates up to **ten** normalized query fragments per request using contiguous word combinations up to length **3**.
- Retains the top **50** matches per level during parallel fragment searches and limits n-gram result materialisation to the same order of magnitude.

Override these defaults by exporting the environment variables listed above or by editing the matcher section in `cmd/api/main.go` / `helm-chart/values.yaml` before deployment.

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
