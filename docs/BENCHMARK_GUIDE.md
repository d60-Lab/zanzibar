# Zanzibar vs MySQL Benchmark - Usage Guide

## Quick Start

### 1. Setup Database

```bash
# Create database
mysql -u root -p -e "CREATE DATABASE gin_template;"

# Run migrations
mysql -u root -p gin_template < migrations/001_permission_comparison_schema.sql
```

### 2. Generate Test Data

```bash
# Set database credentials
export DATABASE_DSN="root:password@tcp(localhost:3306)/gin_template?charset=utf8mb4&parseTime=True&loc=Local"

# Generate full test dataset (WARNING: takes several hours!)
go run cmd/benchmark/main.go generate
```

### 3. Run Benchmarks

```bash
# Quick benchmark (for testing)
go run cmd/benchmark/main.go quick

# Standard benchmark
go run cmd/benchmark/main.go

# Full benchmark (more iterations)
go run cmd/benchmark/main.go full
```

## Test Data Generation

The test data generator creates:

- **10,000 users** with multi-department affiliation
- **~2,000 departments** in 5-level hierarchy
- **100,000 customers** with 1-10 followers each
- **~500,000 documents** with Zipfian distribution
- **10M+ MySQL permission rows** (expanded storage)
- **~1M Zanzibar tuples** (graph storage)

Generation time: 2-6 hours depending on hardware

## Benchmark Categories

### Category A: Single Permission Check
Tests basic permission lookup performance
- **Iterations**: 1,000
- **Operations**:
  - MySQL: Indexed SELECT
  - Zanzibar (Cold): Graph traversal without cache
  - Zanzibar (Warm): Graph traversal with LRU cache

### Category B: Batch Permission Check
Tests checking permissions for 50 documents at once
- **Iterations**: 100
- **Use Case**: Document list view with permission indicators

### Category C: User Document List
Tests fetching paginated list of user's accessible documents
- **Iterations**: 50
- **Page Size**: 20 documents
- **Use Case**: "My Documents" page

### Category D: Single Relationship Change
Tests granting direct permission
- **Iterations**: 50
- **Operations**:
  - Grant direct permission to user

### Category E: Batch Maintenance Operations
Tests department manager change
- **Iterations**: 10
- **Operations**:
  - Zanzibar: Single tuple update
  - MySQL: ⚠️ SKIPPED (would rebuild millions of rows)

### Category F: Concurrent Load
Tests 10 concurrent users checking permissions
- **Iterations**: 100 per worker
- **Concurrency**: 10 workers
- **Total Operations**: 1,000

### Category G: Data Volume Impact
Tests performance with different dataset sizes
- **Sizes**: 10, 50, 100, 500, 1,000 users/documents
- **Use Case**: Scalability analysis

### Category H: Organizational Restructuring
Tests adding users to departments
- **Iterations**: 10
- **Operations**: Add user to department
- **MySQL**: ⚠️ SKIPPED (would require permission rebuild)

### Category I: Customer Team Changes
Tests adding customer followers
- **Iterations**: 10
- **Operations**: Add customer follower
- **Impact**: Affects ALL customer documents

## Understanding Results

### Output Files

After running benchmarks, you'll find:

```
benchmark-results/
├── detailed_results_20060102_150405.csv    # All raw data
├── detailed_results_20060102_150405.json   # All raw data (JSON)
└── summary_20060102_150405.md              # Human-readable summary
```

### Key Metrics

- **Mean**: Average execution time
- **Median**: Middle value (50th percentile)
- **P95**: 95th percentile (95% of requests faster than this)
- **P99**: 99th percentile (99% of requests faster than this)
- **Min/Max**: Range of values

### Expected Performance

Based on design predictions:

| Operation | MySQL | Zanzibar (Cold) | Zanzibar (Warm) |
|-----------|-------|-----------------|-----------------|
| Single Check | 1-5ms | 5-20ms | <1ms |
| Batch (50) | 10-50ms | 50-200ms | 5-10ms |
| User Docs | 50-200ms | 100-500ms | 20-50ms |
| Dept Manager | 10-60s | <100ms | <100ms |
| Customer Follower | 30-120s | <10ms | <10ms |

## API Testing

You can also test via REST API:

### Start the Server

```bash
go run cmd/server/main.go
```

### Test Endpoints

```bash
# Check permission (MySQL)
curl -X POST http://localhost:8080/api/v1/permissions/mysql/check \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-1",
    "document_id": "doc-1",
    "permission_type": "viewer"
  }'

# Check permission (Zanzibar)
curl -X POST http://localhost:8080/api/v1/permissions/zanzibar/check \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-1",
    "document_id": "doc-1",
    "permission_type": "viewer"
  }'

# Compare both engines
curl -X POST http://localhost:8080/api/v1/permissions/both/check \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-1",
    "document_id": "doc-1",
    "permission_type": "viewer"
  }'

# Get storage comparison
curl http://localhost:8080/api/v1/comparison/storage

# Get user documents (MySQL)
curl http://localhost:8080/api/v1/permissions/mysql/users/user-1/documents?permission_type=viewer&page=1&page_size=20

# Get user documents (Zanzibar)
curl http://localhost:8080/api/v1/permissions/zanzibar/users/user-1/documents?permission_type=viewer&page=1&page_size=20

# Update department manager (MySQL - slow!)
curl -X POST http://localhost:8080/api/v1/permissions/mysql/department/manager \
  -H "Content-Type: application/json" \
  -d '{
    "department_id": "dept-l3-0-0-0",
    "manager_id": "user-100"
  }'

# Update department manager (Zanzibar - fast!)
curl -X POST http://localhost:8080/api/v1/permissions/zanzibar/department/manager \
  -H "Content-Type: application/json" \
  -d '{
    "department_id": "dept-l3-0-0-0",
    "manager_id": "user-100"
  }'

# Clear Zanzibar cache
curl -X POST http://localhost:8080/api/v1/permissions/zanzibar/cache/clear
```

## Troubleshooting

### Out of Memory

If generating test data causes OOM:
- Reduce `NumDocuments` in `DefaultConfig()`
- Reduce `BatchSize`
- Increase swap space

### Slow Performance

MySQL permission expansion is intentionally slow!
- This demonstrates the pain point
- Zanzibar should be 100-1000x faster on maintenance ops

### Connection Issues

```bash
# Check MySQL is running
mysql -u root -p -e "SELECT 1"

# Check database exists
mysql -u root -p -e "SHOW DATABASES LIKE 'gin_template'"

# Check tables
mysql -u root -p gin_template -e "SHOW TABLES"
```

## Development

### Running Unit Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific test
go test -v ./internal/repository -run TestMySQLPermissionRepository_CheckPermission
```

### Adding New Benchmarks

1. Add method to `BenchmarkSuite` in `internal/service/benchmark_suite.go`
2. Call from `RunAllBenchmarks()`
3. Use `recordResult()` to log metrics
4. Rebuild and run

## Performance Tips

### For Faster Data Generation

```go
config := GenerateConfig{
    NumUsers:      1000,    // Reduce from 10,000
    NumCustomers:  10000,   // Reduce from 100,000
    NumDocuments:  50000,   // Reduce from 500,000
    BatchSize:     5000,    // Increase for faster inserts
}
```

### For Faster Benchmarks

```bash
# Use quick mode
go run cmd/benchmark/main.go quick

# Or modify config in code:
config.WarmupRounds = 5
config.TestRounds = 50
```

## Architecture Decision Records

### Why SQLite for Unit Tests?
- Fast: In-memory database
- Isolated: Each test gets fresh database
- No external dependencies

### Why MySQL for Benchmarks?
- Realistic: Production database
- InnoDB: Proper transactional behavior
- Indexes: Test actual query plans

### Why 9 Benchmark Categories?
- **Read performance** (A-C): Most common operations
- **Write performance** (D-E): Maintenance operations
- **Scalability** (F-G): Growth planning
- **Real-world** (H-I): Actual use cases

## Next Steps

1. ✅ Run benchmarks with generated data
2. ✅ Analyze CSV/JSON results
3. ✅ Create visualizations (charts/graphs)
4. ✅ Write technical report
5. ✅ Draft CSDN article

## Support

For issues or questions:
- Check `docs/implementation-progress.md` for architecture details
- Review `docs/plans/2025-01-29-*.md` for design decisions
- See unit tests for usage examples
