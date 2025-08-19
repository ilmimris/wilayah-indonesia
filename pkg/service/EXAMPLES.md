# Service Package Examples

This document provides various examples of how to use the service package in different contexts and scenarios.

## Basic Usage Examples

### 1. Simple Search Application

This example shows how to create a simple command-line application that uses the service package:

```go
package main

import (
    "database/sql"
    "fmt"
    "log"
    "os"
    
    "my-project/service" // Adjust import path as needed
    _ "github.com/marcboeker/go-duckdb"
)

func main() {
    if len(os.Args) < 2 {
        log.Fatal("Usage: search-app <query>")
    }
    
    query := os.Args[1]
    
    // Open database connection
    db, err := sql.Open("duckdb", "data/regions.duckdb?access_mode=read_only")
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Create service instance
    svc := service.New(db)
    
    // Perform search
    regions, err := svc.Search(query)
    if err != nil {
        log.Fatal("Search failed:", err)
    }
    
    // Display results
    fmt.Printf("Found %d regions:\n", len(regions))
    for _, region := range regions {
        fmt.Printf("- %s, %s, %s (%s)\n", 
            region.Subdistrict, region.District, region.City, region.PostalCode)
    }
}
```

### 2. Web Service Integration

This example shows how to integrate the service package into a web service:

```go
package main

import (
    "database/sql"
    "encoding/json"
    "log"
    "net/http"
    "os"
    
    "my-project/service" // Adjust import path as needed
    _ "github.com/marcboeker/go-duckdb"
)

type SearchHandler struct {
    svc *service.Service
}

func NewSearchHandler(svc *service.Service) *SearchHandler {
    return &SearchHandler{svc: svc}
}

func (h *SearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    query := r.URL.Query().Get("q")
    if query == "" {
        http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
        return
    }
    
    regions, err := h.svc.Search(query)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(regions)
}

func main() {
    // Open database connection
    dbPath := os.Getenv("DB_PATH")
    if dbPath == "" {
        dbPath = "data/regions.duckdb"
    }
    
    db, err := sql.Open("duckdb", dbPath+"?access_mode=read_only")
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Create service instance
    svc := service.New(db)
    
    // Create handler
    handler := NewSearchHandler(svc)
    
    // Set up routes
    http.Handle("/search", handler)
    
    // Start server
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }
    
    log.Printf("Server starting on port %s", port)
    log.Fatal(http.ListenAndServe(":"+port, nil))
}
```

## Advanced Usage Examples

### 3. Batch Processing Application

This example shows how to use the service package for batch processing:

```go
package main

import (
    "database/sql"
    "encoding/csv"
    "log"
    "os"
    
    "my-project/service" // Adjust import path as needed
    _ "github.com/marcboeker/go-duckdb"
)

func main() {
    // Open database connection
    db, err := sql.Open("duckdb", "data/regions.duckdb?access_mode=read_only")
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Create service instance
    svc := service.New(db)
    
    // Open input CSV file
    file, err := os.Open("input.csv")
    if err != nil {
        log.Fatal("Failed to open input file:", err)
    }
    defer file.Close()
    
    // Read CSV
    reader := csv.NewReader(file)
    records, err := reader.ReadAll()
    if err != nil {
        log.Fatal("Failed to read CSV:", err)
    }
    
    // Open output CSV file
    outputFile, err := os.Create("output.csv")
    if err != nil {
        log.Fatal("Failed to create output file:", err)
    }
    defer outputFile.Close()
    
    writer := csv.NewWriter(outputFile)
    defer writer.Flush()
    
    // Write header
    writer.Write([]string{"input", "id", "subdistrict", "district", "city", "province", "postal_code"})
    
    // Process each record
    for _, record := range records {
        if len(record) == 0 {
            continue
        }
        
        query := record[0]
        regions, err := svc.Search(query)
        if err != nil {
            log.Printf("Search failed for %s: %v", query, err)
            continue
        }
        
        // Write results
        for _, region := range regions {
            writer.Write([]string{
                query,
                region.ID,
                region.Subdistrict,
                region.District,
                region.City,
                region.Province,
                region.PostalCode,
            })
        }
    }
    
    log.Println("Batch processing completed")
}
```

### 4. Interactive CLI Application

This example shows how to create an interactive command-line application:

```go
package main

import (
    "database/sql"
    "fmt"
    "log"
    "os"
    "strings"
    
    "my-project/service" // Adjust import path as needed
    _ "github.com/marcboeker/go-duckdb"
)

func main() {
    // Open database connection
    db, err := sql.Open("duckdb", "data/regions.duckdb?access_mode=read_only")
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Create service instance
    svc := service.New(db)
    
    fmt.Println("Indonesian Regions Search")
    fmt.Println("Type 'quit' to exit")
    fmt.Println()
    
    // Interactive loop
    for {
        fmt.Print("Enter search query: ")
        var input string
        fmt.Scanln(&input)
        
        // Trim whitespace and check for quit command
        input = strings.TrimSpace(input)
        if input == "quit" || input == "exit" {
            fmt.Println("Goodbye!")
            break
        }
        
        if input == "" {
            fmt.Println("Please enter a search query")
            continue
        }
        
        // Perform search
        regions, err := svc.Search(input)
        if err != nil {
            fmt.Printf("Search failed: %v\n", err)
            continue
        }
        
        // Display results
        if len(regions) == 0 {
            fmt.Println("No regions found")
        } else {
            fmt.Printf("Found %d regions:\n", len(regions))
            for i, region := range regions {
                fmt.Printf("%d. %s, %s, %s (%s)\n", 
                    i+1, region.Subdistrict, region.District, region.City, region.PostalCode)
            }
        }
        fmt.Println()
    }
}
```

## Specialized Search Examples

### 5. District-Specific Search

This example shows how to use the district-specific search functionality:

```go
package main

import (
    "database/sql"
    "fmt"
    "log"
    "os"
    
    "my-project/service" // Adjust import path as needed
    _ "github.com/marcboeker/go-duckdb"
)

func main() {
    if len(os.Args) < 2 {
        log.Fatal("Usage: district-search <district-name>")
    }
    
    district := os.Args[1]
    
    // Open database connection
    db, err := sql.Open("duckdb", "data/regions.duckdb?access_mode=read_only")
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Create service instance
    svc := service.New(db)
    
    // Perform district search
    regions, err := svc.SearchByDistrict(district)
    if err != nil {
        log.Fatal("District search failed:", err)
    }
    
    // Display results
    fmt.Printf("Found %d regions in district '%s':\n", len(regions), district)
    for _, region := range regions {
        fmt.Printf("- %s, %s, %s (%s)\n", 
            region.Subdistrict, region.District, region.City, region.PostalCode)
    }
}
```

### 6. Postal Code Lookup

This example shows how to use the postal code search functionality:

```go
package main

import (
    "database/sql"
    "fmt"
    "log"
    "os"
    
    "my-project/service" // Adjust import path as needed
    _ "github.com/marcboeker/go-duckdb"
)

func main() {
    if len(os.Args) < 2 {
        log.Fatal("Usage: postal-search <postal-code>")
    }
    
    postalCode := os.Args[1]
    
    // Open database connection
    db, err := sql.Open("duckdb", "data/regions.duckdb?access_mode=read_only")
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Create service instance
    svc := service.New(db)
    
    // Perform postal code search
    regions, err := svc.SearchByPostalCode(postalCode)
    if err != nil {
        log.Fatal("Postal code search failed:", err)
    }
    
    // Display results
    fmt.Printf("Found %d regions with postal code '%s':\n", len(regions), postalCode)
    for _, region := range regions {
        fmt.Printf("- %s, %s, %s, %s\n", 
            region.Subdistrict, region.District, region.City, region.Province)
    }
}
```

## Integration with Other Systems

### 7. Integration with Logging System

This example shows how to integrate the service with a structured logging system:

```go
package main

import (
    "database/sql"
    "log/slog"
    "os"
    
    "my-project/service" // Adjust import path as needed
    _ "github.com/marcboeker/go-duckdb"
)

func main() {
    // Set up structured logging
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
    slog.SetDefault(logger)
    
    // Open database connection
    db, err := sql.Open("duckdb", "data/regions.duckdb?access_mode=read_only")
    if err != nil {
        slog.Error("Failed to open database", "error", err)
        os.Exit(1)
    }
    defer db.Close()
    
    // Create service instance
    svc := service.New(db)
    
    // Perform search with logging
    slog.Info("Starting search", "query", "jakarta")
    regions, err := svc.Search("jakarta")
    if err != nil {
        slog.Error("Search failed", "error", err)
        os.Exit(1)
    }
    
    slog.Info("Search completed", "results", len(regions))
    
    // Process results
    for _, region := range regions {
        slog.Info("Found region", 
            "subdistrict", region.Subdistrict,
            "district", region.District,
            "city", region.City)
    }
}
```

### 8. Integration with Configuration System

This example shows how to integrate the service with a configuration system:

```go
package main

import (
    "database/sql"
    "encoding/json"
    "log"
    "os"
    
    "my-project/service" // Adjust import path as needed
    _ "github.com/marcboeker/go-duckdb"
)

type Config struct {
    DatabasePath string `json:"database_path"`
    Port         string `json:"port"`
}

func loadConfig(filename string) (*Config, error) {
    file, err := os.Open(filename)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    
    var config Config
    decoder := json.NewDecoder(file)
    err = decoder.Decode(&config)
    return &config, err
}

func main() {
    // Load configuration
    config, err := loadConfig("config.json")
    if err != nil {
        log.Fatal("Failed to load config:", err)
    }
    
    // Open database connection
    db, err := sql.Open("duckdb", config.DatabasePath+"?access_mode=read_only")
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Create service instance
    svc := service.New(db)
    
    // Perform search
    regions, err := svc.Search("jakarta")
    if err != nil {
        log.Fatal("Search failed:", err)
    }
    
    log.Printf("Found %d regions", len(regions))
}
```

## Error Handling Examples

### 9. Comprehensive Error Handling

This example shows how to handle different types of errors from the service:

```go
package main

import (
    "database/sql"
    "fmt"
    "log"
    "os"
    
    "my-project/service" // Adjust import path as needed
    _ "github.com/marcboeker/go-duckdb"
)

func main() {
    if len(os.Args) < 2 {
        log.Fatal("Usage: search-app <query>")
    }
    
    query := os.Args[1]
    
    // Open database connection
    db, err := sql.Open("duckdb", "data/regions.duckdb?access_mode=read_only")
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Create service instance
    svc := service.New(db)
    
    // Perform search with comprehensive error handling
    regions, err := svc.Search(query)
    if err != nil {
        // Handle specific error types
        switch {
        case service.IsError(err, service.ErrCodeInvalidInput):
            log.Fatal("Invalid input:", err)
        case service.IsError(err, service.ErrCodeNotFound):
            log.Println("No regions found for the query")
            return
        case service.IsError(err, service.ErrCodeDatabaseFailure):
            log.Fatal("Database error:", err)
        default:
            log.Fatal("Unexpected error:", err)
        }
    }
    
    // Display results
    fmt.Printf("Found %d regions:\n", len(regions))
    for _, region := range regions {
        fmt.Printf("- %s, %s, %s (%s)\n", 
            region.Subdistrict, region.District, region.City, region.PostalCode)
    }
}
```

## Performance Optimization Examples

### 10. Caching Implementation

This example shows how to implement caching to improve performance:

```go
package main

import (
    "database/sql"
    "fmt"
    "log"
    "os"
    "sync"
    "time"
    
    "my-project/service" // Adjust import path as needed
    _ "github.com/marcboeker/go-duckdb"
)

type CachedService struct {
    svc   *service.Service
    cache map[string][]service.Region
    mutex sync.RWMutex
    ttl   time.Duration
}

func NewCachedService(svc *service.Service, ttl time.Duration) *CachedService {
    return &CachedService{
        svc:   svc,
        cache: make(map[string][]service.Region),
        ttl:   ttl,
    }
}

func (cs *CachedService) Search(query string) ([]service.Region, error) {
    // Check cache first
    cs.mutex.RLock()
    if regions, exists := cs.cache[query]; exists {
        cs.mutex.RUnlock()
        fmt.Printf("Cache hit for query: %s\n", query)
        return regions, nil
    }
    cs.mutex.RUnlock()
    
    // Cache miss, perform actual search
    fmt.Printf("Cache miss for query: %s\n", query)
    regions, err := cs.svc.Search(query)
    if err != nil {
        return nil, err
    }
    
    // Store in cache
    cs.mutex.Lock()
    cs.cache[query] = regions
    cs.mutex.Unlock()
    
    // Set up cache expiration
    go func() {
        time.Sleep(cs.ttl)
        cs.mutex.Lock()
        delete(cs.cache, query)
        cs.mutex.Unlock()
        fmt.Printf("Cache expired for query: %s\n", query)
    }()
    
    return regions, nil
}

func main() {
    if len(os.Args) < 2 {
        log.Fatal("Usage: cached-search <query>")
    }
    
    query := os.Args[1]
    
    // Open database connection
    db, err := sql.Open("duckdb", "data/regions.duckdb?access_mode=read_only")
    if err != nil {
        log.Fatal("Failed to open database:", err)
    }
    defer db.Close()
    
    // Create service instance
    svc := service.New(db)
    
    // Create cached service with 5-minute TTL
    cachedSvc := NewCachedService(svc, 5*time.Minute)
    
    // Perform search (first time)
    fmt.Println("First search:")
    regions, err := cachedSvc.Search(query)
    if err != nil {
        log.Fatal("Search failed:", err)
    }
    
    fmt.Printf("Found %d regions\n", len(regions))
    
    // Perform search again (should be cached)
    fmt.Println("\nSecond search (should be cached):")
    regions, err = cachedSvc.Search(query)
    if err != nil {
        log.Fatal("Search failed:", err)
    }
    
    fmt.Printf("Found %d regions\n", len(regions))
}
```

## Testing Examples

### 11. Unit Testing with Mock Database

This example shows how to write unit tests for code that uses the service package:

```go
package main

import (
    "database/sql"
    "testing"
    
    "my-project/service" // Adjust import path as needed
    _ "github.com/marcboeker/go-duckdb"
)

// Mock database setup for testing
func setupTestDB(t *testing.T) *sql.DB {
    db, err := sql.Open("duckdb", "test.duckdb")
    if err != nil {
        t.Fatalf("Failed to open test database: %v", err)
    }
    
    // Create test schema
    _, err = db.Exec(`
        CREATE TABLE regions (
            id VARCHAR,
            subdistrict VARCHAR,
            district VARCHAR,
            city VARCHAR,
            province VARCHAR,
            postal_code VARCHAR,
            full_text VARCHAR
        )
    `)
    if err != nil {
        t.Fatalf("Failed to create test schema: %v", err)
    }
    
    // Insert test data
    _, err = db.Exec(`
        INSERT INTO regions (id, subdistrict, district, city, province, postal_code, full_text)
        VALUES ('1', 'Test Subdistrict', 'Test District', 'Test City', 'Test Province', '12345', 'test province test city test district test subdistrict')
    `)
    if err != nil {
        t.Fatalf("Failed to insert test data: %v", err)
    }
    
    return db
}

func TestSearchFunctionality(t *testing.T) {
    // Set up test database
    db := setupTestDB(t)
    defer db.Close()
    
    // Create service instance
    svc := service.New(db)
    
    // Test search functionality
    regions, err := svc.Search("test")
    if err != nil {
        t.Fatalf("Search failed: %v", err)
    }
    
    // Verify results
    if len(regions) != 1 {
        t.Errorf("Expected 1 region, got %d", len(regions))
    }
    
    if len(regions) > 0 {
        region := regions[0]
        if region.Subdistrict != "Test Subdistrict" {
            t.Errorf("Expected 'Test Subdistrict', got '%s'", region.Subdistrict)
        }
    }
}

func TestInvalidInput(t *testing.T) {
    // Set up test database
    db := setupTestDB(t)
    defer db.Close()
    
    // Create service instance
    svc := service.New(db)
    
    // Test with empty query
    _, err := svc.Search("")
    if err == nil {
        t.Error("Expected error for empty query, got nil")
    }
    
    // Verify error type
    if !service.IsError(err, service.ErrCodeInvalidInput) {
        t.Errorf("Expected ErrCodeInvalidInput, got %v", err)
    }
}
```

## Conclusion

These examples demonstrate various ways to use the service package in different contexts:

1. **Basic Usage**: Simple command-line applications and web services
2. **Advanced Usage**: Batch processing and interactive applications
3. **Specialized Search**: District-specific and postal code searches
4. **Integration**: With logging and configuration systems
5. **Error Handling**: Comprehensive error handling patterns
6. **Performance**: Caching implementations
7. **Testing**: Unit testing with mock databases

By following these examples, you can adapt the service package to fit your specific use cases and requirements.