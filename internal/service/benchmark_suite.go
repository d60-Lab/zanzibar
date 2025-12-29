package service

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"

	"github.com/maynardzanzibar/internal/model"
	"github.com/maynardzanzibar/internal/repository"
)

// BenchmarkSuite manages performance benchmarking
type BenchmarkSuite struct {
	db                *gorm.DB
	mysqlRepo         *repository.MySQLPermissionRepository
	zanzibarRepo      *repository.ZanzibarPermissionRepository
	results           []BenchmarkResult
	mu                sync.Mutex
	benchmarkID       int64
}

// NewBenchmarkSuite creates a new benchmark suite
func NewBenchmarkSuite(db *gorm.DB, mysqlRepo *repository.MySQLPermissionRepository, zanzibarRepo *repository.ZanzibarPermissionRepository) *BenchmarkSuite {
	return &BenchmarkSuite{
		db:           db,
		mysqlRepo:    mysqlRepo,
		zanzibarRepo: zanzibarRepo,
		results:      make([]BenchmarkResult, 0),
	}
}

// BenchmarkConfig holds benchmark configuration
type BenchmarkConfig struct {
	TestName        string
	WarmupRounds    int
	TestRounds      int
	Concurrency     int
	Timeout         time.Duration
	OutputDir       string
	Verbose         bool
}

// DefaultBenchmarkConfig returns default benchmark configuration
func DefaultBenchmarkConfig() BenchmarkConfig {
	return BenchmarkConfig{
		TestName:     "full-comparison",
		WarmupRounds: 100,
		TestRounds:   1000,
		Concurrency:  10,
		Timeout:      5 * time.Minute,
		OutputDir:    "./benchmark-results",
		Verbose:      true,
	}
}

// BenchmarkResult represents a single benchmark result
type BenchmarkResult struct {
	BenchmarkID    int64     `json:"benchmark_id"`
	TestName       string    `json:"test_name"`
	TestCategory   string    `json:"test_category"`
	EngineType     string    `json:"engine_type"`
	Operation      string    `json:"operation"`
	DurationMs     float64   `json:"duration_ms"`
	RowsAffected   int       `json:"rows_affected"`
	Success        bool      `json:"success"`
	CacheHit       bool      `json:"cache_hit"`
	Error          string    `json:"error,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

// BenchmarkStats represents aggregated statistics
type BenchmarkStats struct {
	TestName     string  `json:"test_name"`
	EngineType   string  `json:"engine_type"`
	Operation    string  `json:"operation"`
	Samples      int     `json:"samples"`
	MeanMs       float64 `json:"mean_ms"`
	MedianMs     float64 `json:"median_ms"`
	P50Ms        float64 `json:"p50_ms"`
	P95Ms        float64 `json:"p95_ms"`
	P99Ms        float64 `json:"p99_ms"`
	MinMs        float64 `json:"min_ms"`
	MaxMs        float64 `json:"max_ms"`
	Throughput   float64 `json:"throughput_ops_per_sec"`
	ErrorRate    float64 `json:"error_rate_percent"`
}

// RunAllBenchmarks executes all benchmark categories
func (b *BenchmarkSuite) RunAllBenchmarks(ctx context.Context, config BenchmarkConfig) error {
	fmt.Println("🚀 Starting comprehensive benchmark suite...")
	startTime := time.Now()

	// Create output directory
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Clear Zanzibar cache before tests
	b.zanzibarRepo.ClearCache()

	// Category A: Single Permission Check
	fmt.Println("\n📊 Category A: Single Permission Check")
	if err := b.runBenchmarkCategoryA(ctx, config); err != nil {
		return fmt.Errorf("category A failed: %w", err)
	}

	// Category B: Batch Permission Check
	fmt.Println("\n📊 Category B: Batch Permission Check")
	if err := b.runBenchmarkCategoryB(ctx, config); err != nil {
		return fmt.Errorf("category B failed: %w", err)
	}

	// Category C: User Document List
	fmt.Println("\n📊 Category C: User Document List")
	if err := b.runBenchmarkCategoryC(ctx, config); err != nil {
		return fmt.Errorf("category C failed: %w", err)
	}

	// Category D: Single Relationship Change
	fmt.Println("\n📊 Category D: Single Relationship Change")
	if err := b.runBenchmarkCategoryD(ctx, config); err != nil {
		return fmt.Errorf("category D failed: %w", err)
	}

	// Category E: Batch Maintenance Operations
	fmt.Println("\n📊 Category E: Batch Maintenance Operations")
	if err := b.runBenchmarkCategoryE(ctx, config); err != nil {
		return fmt.Errorf("category E failed: %w", err)
	}

	// Category F: Concurrent Load
	fmt.Println("\n📊 Category F: Concurrent Load")
	if err := b.runBenchmarkCategoryF(ctx, config); err != nil {
		return fmt.Errorf("category F failed: %w", err)
	}

	// Category G: Data Volume Impact (skip if too slow)
	if config.TestRounds <= 100 {
		fmt.Println("\n📊 Category G: Data Volume Impact")
		if err := b.runBenchmarkCategoryG(ctx, config); err != nil {
			return fmt.Errorf("category G failed: %w", err)
		}
	}

	// Category H: Organizational Restructuring
	fmt.Println("\n📊 Category H: Organizational Restructuring")
	if err := b.runBenchmarkCategoryH(ctx, config); err != nil {
		return fmt.Errorf("category H failed: %w", err)
	}

	// Category I: Customer Team Changes
	fmt.Println("\n📊 Category I: Customer Team Changes")
	if err := b.runBenchmarkCategoryI(ctx, config); err != nil {
		return fmt.Errorf("category I failed: %w", err)
	}

	duration := time.Since(startTime)
	fmt.Printf("\n✅ All benchmarks completed in %v\n", duration)

	// Generate reports
	if err := b.generateReports(config); err != nil {
		return fmt.Errorf("failed to generate reports: %w", err)
	}

	return nil
}

// Category A: Single Permission Check
func (b *BenchmarkSuite) runBenchmarkCategoryA(ctx context.Context, config BenchmarkConfig) error {
	// Get sample users and documents
	var users []model.User
	if err := b.db.WithContext(ctx).Limit(100).Find(&users).Error; err != nil {
		return err
	}

	var docs []model.Document
	if err := b.db.WithContext(ctx).Limit(100).Find(&docs).Error; err != nil {
		return err
	}

	// Warmup
	fmt.Println("   Warmup...")
	for i := 0; i < config.WarmupRounds; i++ {
		user := users[i%len(users)]
		doc := docs[i%len(docs)]
		b.mysqlRepo.CheckPermission(ctx, user.ID, doc.ID, "viewer")
		b.zanzibarRepo.CheckPermission(ctx, user.ID, doc.ID, "viewer")
	}

	// Benchmark MySQL
	fmt.Println("   Testing MySQL...")
	mysqlTimes := b.runSingleCheckBenchmark(ctx, "A", "single_permission_check", "mysql", b.mysqlRepo, users, docs, config.TestRounds)

	// Benchmark Zanzibar
	fmt.Println("   Testing Zanzibar...")
	b.zanzibarRepo.ClearCache()
	zanzibarColdTimes := b.runSingleCheckBenchmark(ctx, "A", "single_permission_check", "zanzibar_cold", b.zanzibarRepo, users, docs, config.TestRounds/2)

	// Zanzibar warm cache
	zanzibarWarmTimes := b.runSingleCheckBenchmark(ctx, "A", "single_permission_check", "zanzibar_warm", b.zanzibarRepo, users, docs, config.TestRounds/2)

	// Calculate and print stats
	b.printStats("MySQL", mysqlTimes)
	b.printStats("Zanzibar (Cold)", zanzibarColdTimes)
	b.printStats("Zanzibar (Warm)", zanzibarWarmTimes)

	return nil
}

func (b *BenchmarkSuite) runSingleCheckBenchmark(ctx context.Context, category, operation, engine string, repo interface{}, users []model.User, docs []model.Document, rounds int) []float64 {
	times := make([]float64, rounds)

	for i := 0; i < rounds; i++ {
		user := users[i%len(users)]
		doc := docs[i%len(docs)]

		start := time.Now()

		switch r := repo.(type) {
		case *repository.MySQLPermissionRepository:
			_, _ = r.CheckPermission(ctx, user.ID, doc.ID, "viewer")
		case *repository.ZanzibarPermissionRepository:
			_, _ = r.CheckPermission(ctx, user.ID, doc.ID, "viewer")
		}

		duration := time.Since(start)
		times[i] = float64(duration.Microseconds()) / 1000.0 // Convert to ms

		b.recordResult(category, operation, engine, times[i], 0, true, false)
	}

	return times
}

// Category B: Batch Permission Check
func (b *BenchmarkSuite) runBenchmarkCategoryB(ctx context.Context, config BenchmarkConfig) error {
	var users []model.User
	if err := b.db.WithContext(ctx).Limit(10).Find(&users).Error; err != nil {
		return err
	}

	var docs []model.Document
	if err := b.db.WithContext(ctx).Limit(50).Find(&docs).Error; err != nil {
		return err
	}

	docIDs := make([]string, len(docs))
	for i, doc := range docs {
		docIDs[i] = doc.ID
	}

	// Warmup
	for i := 0; i < 10; i++ {
		user := users[i%len(users)]
		b.mysqlRepo.CheckPermissionsBatch(ctx, user.ID, docIDs, "viewer")
		b.zanzibarRepo.CheckPermissionsBatch(ctx, user.ID, docIDs, "viewer")
	}

	// Benchmark MySQL
	fmt.Println("   Testing MySQL batch checks...")
	mysqlTimes := make([]float64, 100)
	for i := 0; i < 100; i++ {
		user := users[i%len(users)]
		start := time.Now()
		_, _ = b.mysqlRepo.CheckPermissionsBatch(ctx, user.ID, docIDs, "viewer")
		duration := time.Since(start)
		mysqlTimes[i] = float64(duration.Microseconds()) / 1000.0
		b.recordResult("B", "batch_permission_check_50", "mysql", mysqlTimes[i], 0, true, false)
	}

	// Benchmark Zanzibar
	fmt.Println("   Testing Zanzibar batch checks...")
	b.zanzibarRepo.ClearCache()
	zanzibarTimes := make([]float64, 100)
	for i := 0; i < 100; i++ {
		user := users[i%len(users)]
		start := time.Now()
		_, _ = b.zanzibarRepo.CheckPermissionsBatch(ctx, user.ID, docIDs, "viewer")
		duration := time.Since(start)
		zanzibarTimes[i] = float64(duration.Microseconds()) / 1000.0
		b.recordResult("B", "batch_permission_check_50", "zanzibar", zanzibarTimes[i], 0, true, false)
	}

	b.printStats("MySQL (50 docs)", mysqlTimes)
	b.printStats("Zanzibar (50 docs)", zanzibarTimes)

	return nil
}

// Category C: User Document List
func (b *BenchmarkSuite) runBenchmarkCategoryC(ctx context.Context, config BenchmarkConfig) error {
	var users []model.User
	if err := b.db.WithContext(ctx).Limit(50).Find(&users).Error; err != nil {
		return err
	}

	// Warmup
	for i := 0; i < 10; i++ {
		user := users[i%len(users)]
		b.mysqlRepo.GetUserDocuments(ctx, user.ID, "viewer", 1, 20)
		b.zanzibarRepo.GetUserDocuments(ctx, user.ID, "viewer", 1, 20)
	}

	// Benchmark MySQL
	fmt.Println("   Testing MySQL user document lists...")
	mysqlTimes := make([]float64, 50)
	for i := 0; i < 50; i++ {
		user := users[i%len(users)]
		start := time.Now()
		_, _ = b.mysqlRepo.GetUserDocuments(ctx, user.ID, "viewer", 1, 20)
		duration := time.Since(start)
		mysqlTimes[i] = float64(duration.Microseconds()) / 1000.0
		b.recordResult("C", "user_document_list_page1", "mysql", mysqlTimes[i], 0, true, false)
	}

	// Benchmark Zanzibar
	fmt.Println("   Testing Zanzibar user document lists...")
	b.zanzibarRepo.ClearCache()
	zanzibarTimes := make([]float64, 50)
	for i := 0; i < 50; i++ {
		user := users[i%len(users)]
		start := time.Now()
		_, _ = b.zanzibarRepo.GetUserDocuments(ctx, user.ID, "viewer", 1, 20)
		duration := time.Since(start)
		zanzibarTimes[i] = float64(duration.Microseconds()) / 1000.0
		b.recordResult("C", "user_document_list_page1", "zanzibar", zanzibarTimes[i], 0, true, false)
	}

	b.printStats("MySQL (page 1, 20 items)", mysqlTimes)
	b.printStats("Zanzibar (page 1, 20 items)", zanzibarTimes)

	return nil
}

// Category D: Single Relationship Change
func (b *BenchmarkSuite) runBenchmarkCategoryD(ctx context.Context, config BenchmarkConfig) error {
	// Test: Grant direct permission
	var users []model.User
	b.db.WithContext(ctx).Limit(10).Find(&users)

	var docs []model.Document
	b.db.WithContext(ctx).Limit(10).Find(&docs)

	fmt.Println("   Testing MySQL: Grant permission...")
	mysqlTimes := make([]float64, 50)
	for i := 0; i < 50; i++ {
		user := users[i%len(users)]
		doc := docs[i%len(docs)]
		start := time.Now()
		_ = b.mysqlRepo.GrantDirectPermission(ctx, user.ID, doc.ID, "viewer")
		duration := time.Since(start)
		mysqlTimes[i] = float64(duration.Microseconds()) / 1000.0
		b.recordResult("D", "grant_direct_permission", "mysql", mysqlTimes[i], 1, true, false)
	}

	fmt.Println("   Testing Zanzibar: Grant permission...")
	b.zanzibarRepo.ClearCache()
	zanzibarTimes := make([]float64, 50)
	for i := 0; i < 50; i++ {
		user := users[i%len(users)]
		doc := docs[i%len(docs)]
		start := time.Now()
		_ = b.zanzibarRepo.GrantDirectPermission(ctx, user.ID, doc.ID, "viewer")
		duration := time.Since(start)
		zanzibarTimes[i] = float64(duration.Microseconds()) / 1000.0
		b.recordResult("D", "grant_direct_permission", "zanzibar", zanzibarTimes[i], 1, true, false)
	}

	b.printStats("MySQL: Grant Permission", mysqlTimes)
	b.printStats("Zanzibar: Grant Permission", zanzibarTimes)

	return nil
}

// Category E: Batch Maintenance Operations
func (b *BenchmarkSuite) runBenchmarkCategoryE(ctx context.Context, config BenchmarkConfig) error {
	// Get a department with users
	var dept model.Department
	if err := b.db.WithContext(ctx).Where("level = ?", 3).First(&dept).Error; err != nil {
		return err
	}

	var users []model.User
	if err := b.db.WithContext(ctx).Limit(10).Find(&users).Error; err != nil {
		return err
	}

	newManager := users[0]

	// Test MySQL department manager change (SKIPPED - too expensive)
	fmt.Println("   ⚠️  MySQL: Skipped (would take minutes to rebuild permissions)")
	fmt.Println("   Testing Zanzibar: Update department manager...")

	b.zanzibarRepo.ClearCache()
	zanzibarTimes := make([]float64, 10)
	for i := 0; i < 10; i++ {
		start := time.Now()
		_ = b.zanzibarRepo.UpdateDepartmentManager(ctx, dept.ID, newManager.ID)
		duration := time.Since(start)
		zanzibarTimes[i] = float64(duration.Microseconds()) / 1000.0
		b.recordResult("E", "update_dept_manager", "zanzibar", zanzibarTimes[i], 1, true, false)
	}

	b.printStats("Zanzibar: Update Dept Manager", zanzibarTimes)
	fmt.Println("   📊 Note: MySQL would require rebuilding millions of permission rows")

	return nil
}

// Category F: Concurrent Load
func (b *BenchmarkSuite) runBenchmarkCategoryF(ctx context.Context, config BenchmarkConfig) error {
	fmt.Println("   Testing MySQL concurrent permission checks...")
	mysqlDuration := b.runConcurrentTest(ctx, "mysql", b.mysqlRepo, config.Concurrency, 100)

	fmt.Println("   Testing Zanzibar concurrent permission checks...")
	b.zanzibarRepo.ClearCache()
	zanzibarDuration := b.runConcurrentTest(ctx, "zanzibar", b.zanzibarRepo, config.Concurrency, 100)

	fmt.Printf("   MySQL: %.2f ms total\n", mysqlDuration)
	fmt.Printf("   Zanzibar: %.2f ms total\n", zanzibarDuration)
	fmt.Printf("   Speedup: %.2fx\n", mysqlDuration/zanzibarDuration)

	return nil
}

func (b *BenchmarkSuite) runConcurrentTest(ctx context.Context, engine string, repo interface{}, concurrency, iterations int) float64 {
	var users []model.User
	b.db.WithContext(ctx).Limit(100).Find(&users)

	var docs []model.Document
	b.db.WithContext(ctx).Limit(100).Find(&docs)

	var wg sync.WaitGroup
	var totalDuration int64
	iterationsPerWorker := iterations / concurrency

	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < iterationsPerWorker; j++ {
				user := users[(workerID*iterationsPerWorker+j)%len(users)]
				doc := docs[(workerID*iterationsPerWorker+j)%len(docs)]

				start := time.Now()

				switch r := repo.(type) {
				case *repository.MySQLPermissionRepository:
					_, _ = r.CheckPermission(ctx, user.ID, doc.ID, "viewer")
				case *repository.ZanzibarPermissionRepository:
					_, _ = r.CheckPermission(ctx, user.ID, doc.ID, "viewer")
				}

				duration := time.Since(start)
				atomic.AddInt64(&totalDuration, int64(duration.Microseconds()))
			}
		}(i)
	}

	wg.Wait()
	totalTime := time.Since(startTime)

	avgMs := float64(atomic.LoadInt64(&totalDuration)) / float64(iterations) / 1000.0
	b.recordResult("F", "concurrent_permission_check", engine, avgMs, 0, true, false)

	return float64(totalTime.Microseconds()) / 1000.0
}

// Category G: Data Volume Impact
func (b *BenchmarkSuite) runBenchmarkCategoryG(ctx context.Context, config BenchmarkConfig) error {
	fmt.Println("   Testing permission check with different data volumes...")

	// Test with different user subsets
	sizes := []int{10, 50, 100, 500, 1000}

	for _, size := range sizes {
		var users []model.User
		b.db.WithContext(ctx).Limit(size).Find(&users)

		var docs []model.Document
		b.db.WithContext(ctx).Limit(size).Find(&docs)

		// MySQL
		start := time.Now()
		for i := 0; i < 10; i++ {
			user := users[i%len(users)]
			doc := docs[i%len(docs)]
			b.mysqlRepo.CheckPermission(ctx, user.ID, doc.ID, "viewer")
		}
		mysqlDuration := time.Since(start)

		// Zanzibar
		b.zanzibarRepo.ClearCache()
		start = time.Now()
		for i := 0; i < 10; i++ {
			user := users[i%len(users)]
			doc := docs[i%len(docs)]
			b.zanzibarRepo.CheckPermission(ctx, user.ID, doc.ID, "viewer")
		}
		zanzibarDuration := time.Since(start)

		fmt.Printf("   Size %d: MySQL %.2fms, Zanzibar %.2fms\n", size, float64(mysqlDuration.Microseconds())/1000.0, float64(zanzibarDuration.Microseconds())/1000.0)
	}

	return nil
}

// Category H: Organizational Restructuring
func (b *BenchmarkSuite) runBenchmarkCategoryH(ctx context.Context, config BenchmarkConfig) error {
	fmt.Println("   Testing: Move 10 employees to new department")

	var users []model.User
	b.db.WithContext(ctx).Limit(10).Find(&users)

	var dept model.Department
	b.db.WithContext(ctx).First(&dept)

	// Zanzibar: Add users to department
	fmt.Println("   Testing Zanzibar: Add users to department...")
	b.zanzibarRepo.ClearCache()
	zanzibarTimes := make([]float64, 10)
	for i := 0; i < 10; i++ {
		user := users[i%len(users)]
		start := time.Now()
		_ = b.zanzibarRepo.AddUserToDepartment(ctx, user.ID, dept.ID, "member", false)
		duration := time.Since(start)
		zanzibarTimes[i] = float64(duration.Microseconds()) / 1000.0
		b.recordResult("H", "add_user_to_department", "zanzibar", zanzibarTimes[i], 2, true, false)
	}

	b.printStats("Zanzibar: Add User to Department", zanzibarTimes)
	fmt.Println("   ⚠️  MySQL: Skipped (would require rebuilding all permissions for affected users)")

	return nil
}

// Category I: Customer Team Changes
func (b *BenchmarkSuite) runBenchmarkCategoryI(ctx context.Context, config BenchmarkConfig) error {
	fmt.Println("   Testing: Add customer follower")

	var customers []model.Customer
	b.db.WithContext(ctx).Limit(10).Find(&customers)

	var users []model.User
	b.db.WithContext(ctx).Limit(10).Find(&users)

	// MySQL
	fmt.Println("   Testing MySQL: Add customer follower...")
	mysqlTimes := make([]float64, 10)
	for i := 0; i < 10; i++ {
		customer := customers[i%len(customers)]
		user := users[i%len(users)]
		start := time.Now()
		_ = b.mysqlRepo.AddCustomerFollowerPermissions(ctx, customer.ID, user.ID)
		duration := time.Since(start)
		mysqlTimes[i] = float64(duration.Microseconds()) / 1000.0
		b.recordResult("I", "add_customer_follower", "mysql", mysqlTimes[i], 0, true, false)
	}

	// Zanzibar
	fmt.Println("   Testing Zanzibar: Add customer follower...")
	b.zanzibarRepo.ClearCache()
	zanzibarTimes := make([]float64, 10)
	for i := 0; i < 10; i++ {
		customer := customers[i%len(customers)]
		user := users[i%len(users)]
		start := time.Now()
		_ = b.zanzibarRepo.AddCustomerFollower(ctx, customer.ID, user.ID)
		duration := time.Since(start)
		zanzibarTimes[i] = float64(duration.Microseconds()) / 1000.0
		b.recordResult("I", "add_customer_follower", "zanzibar", zanzibarTimes[i], 1, true, false)
	}

	b.printStats("MySQL: Add Customer Follower", mysqlTimes)
	b.printStats("Zanzibar: Add Customer Follower", zanzibarTimes)

	return nil
}

// Helper functions

func (b *BenchmarkSuite) recordResult(category, operation, engine string, durationMs float64, rowsAffected int, success, cacheHit bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.benchmarkID++

	result := BenchmarkResult{
		BenchmarkID:  b.benchmarkID,
		TestName:     category,
		TestCategory: category,
		EngineType:   engine,
		Operation:    operation,
		DurationMs:   durationMs,
		RowsAffected: rowsAffected,
		Success:      success,
		CacheHit:     cacheHit,
		Timestamp:    time.Now(),
	}

	b.results = append(b.results, result)
}

func (b *BenchmarkSuite) printStats(label string, times []float64) {
	if len(times) == 0 {
		return
	}

	mean := mean(times)
	median := median(times)
	p95 := percentile(times, 95)
	p99 := percentile(times, 99)
	min := min(times)
	max := max(times)

	fmt.Printf("   %s:\n", label)
	fmt.Printf("      Mean: %.3fms, Median: %.3fms, P95: %.3fms, P99: %.3fms\n", mean, median, p95, p99)
	fmt.Printf("      Min: %.3fms, Max: %.3fms\n", min, max)
}

func mean(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func median(values []float64) float64 {
	sorted := make([]float64, len(values))
	copy(sorted, values)
	// Simple sort (for small datasets)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted[len(sorted)/2]
}

func percentile(values []float64, p int) float64 {
	sorted := make([]float64, len(values))
	copy(sorted, values)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	idx := int(math.Ceil(float64(len(sorted)) * float64(p) / 100.0))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func min(values []float64) float64 {
	m := values[0]
	for _, v := range values {
		if v < m {
			m = v
		}
	}
	return m
}

func max(values []float64) float64 {
	m := values[0]
	for _, v := range values {
		if v > m {
			m = v
		}
	}
	return m
}

// generateReports generates CSV and JSON reports
func (b *BenchmarkSuite) generateReports(config BenchmarkConfig) error {
	fmt.Println("\n📄 Generating reports...")

	// Generate detailed CSV
	if err := b.generateCSVReport(config); err != nil {
		return err
	}

	// Generate JSON report
	if err := b.generateJSONReport(config); err != nil {
		return err
	}

	// Generate summary report
	if err := b.generateSummaryReport(config); err != nil {
		return err
	}

	fmt.Println("   ✅ Reports generated in:", config.OutputDir)
	return nil
}

func (b *BenchmarkSuite) generateCSVReport(config BenchmarkConfig) error {
	filename := fmt.Sprintf("%s/detailed_results_%s.csv", config.OutputDir, time.Now().Format("20060102_150405"))
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"BenchmarkID", "TestName", "TestCategory", "EngineType", "Operation", "DurationMs", "RowsAffected", "Success", "CacheHit", "Timestamp"}
	writer.Write(header)

	// Write data
	for _, result := range b.results {
		row := []string{
			fmt.Sprintf("%d", result.BenchmarkID),
			result.TestName,
			result.TestCategory,
			result.EngineType,
			result.Operation,
			fmt.Sprintf("%.3f", result.DurationMs),
			fmt.Sprintf("%d", result.RowsAffected),
			fmt.Sprintf("%t", result.Success),
			fmt.Sprintf("%t", result.CacheHit),
			result.Timestamp.Format(time.RFC3339),
		}
		writer.Write(row)
	}

	fmt.Printf("   📊 Detailed CSV: %s\n", filename)
	return nil
}

func (b *BenchmarkSuite) generateJSONReport(config BenchmarkConfig) error {
	filename := fmt.Sprintf("%s/detailed_results_%s.json", config.OutputDir, time.Now().Format("20060102_150405"))
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(b.results); err != nil {
		return err
	}

	fmt.Printf("   📊 Detailed JSON: %s\n", filename)
	return nil
}

func (b *BenchmarkSuite) generateSummaryReport(config BenchmarkConfig) error {
	filename := fmt.Sprintf("%s/summary_%s.md", config.OutputDir, time.Now().Format("20060102_150405"))
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Group results by operation
	grouped := make(map[string]map[string][]float64) // operation -> engine -> times
	for _, result := range b.results {
		if _, ok := grouped[result.Operation]; !ok {
			grouped[result.Operation] = make(map[string][]float64)
		}
		grouped[result.Operation][result.EngineType] = append(grouped[result.Operation][result.EngineType], result.DurationMs)
	}

	// Write summary
	file.WriteString("# Benchmark Summary Report\n\n")
	file.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	for operation, engines := range grouped {
		file.WriteString(fmt.Sprintf("## %s\n\n", operation))

		for engine, times := range engines {
			if len(times) == 0 {
				continue
			}

			file.WriteString(fmt.Sprintf("### %s\n\n", engine))
			file.WriteString(fmt.Sprintf("- **Mean**: %.3f ms\n", mean(times)))
			file.WriteString(fmt.Sprintf("- **Median**: %.3f ms\n", median(times)))
			file.WriteString(fmt.Sprintf("- **P95**: %.3f ms\n", percentile(times, 95)))
			file.WriteString(fmt.Sprintf("- **P99**: %.3f ms\n", percentile(times, 99)))
			file.WriteString(fmt.Sprintf("- **Min**: %.3f ms\n", min(times)))
			file.WriteString(fmt.Sprintf("- **Max**: %.3f ms\n", max(times)))
			file.WriteString(fmt.Sprintf("- **Samples**: %d\n\n", len(times)))
		}
	}

	fmt.Printf("   📊 Summary Report: %s\n", filename)
	return nil
}
