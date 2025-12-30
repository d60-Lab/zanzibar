package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/d60-Lab/gin-template/internal/repository"
	"github.com/d60-Lab/gin-template/internal/service"
)

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║       Quick Test: Small Dataset for Demo                  ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Quick test configuration
	dsn := "root:123456@tcp(127.0.0.1:3306)/zanzibar_permission?charset=utf8mb4&parseTime=True&loc=Local"

	// Connect to database
	fmt.Println("🔌 Connecting to database...")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get database instance: %v", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("✅ Database connection established")
	fmt.Println()

	// Check if we need to generate data
	if len(os.Args) > 1 && os.Args[1] == "generate" {
		fmt.Println("🎲 Generating SMALL test dataset for quick demo...")
		fmt.Println()

		generator := service.NewTestDataGenerator(db)

		// SMALL configuration for quick testing
		config := service.GenerateConfig{
			NumUsers:          500,          // Small: 500 users (vs 10,000)
			NumDepartments:    100,          // Small: 100 depts (vs 2,000)
			NumCustomers:      5000,         // Small: 5K customers (vs 100K)
			NumDocuments:      25000,        // Small: 25K docs (vs 500K)
			MaxDeptLevels:     5,
			MaxDeptMembers:    50,
			MaxCustomerFollowers: 10,
			BatchSize:         1000,
		}

		fmt.Printf("Configuration:\n")
		fmt.Printf("  Users:     %d\n", config.NumUsers)
		fmt.Printf("  Depts:     %d\n", config.NumDepartments)
		fmt.Printf("  Customers: %d\n", config.NumCustomers)
		fmt.Printf("  Documents: %d\n", config.NumDocuments)
		fmt.Println()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		startTime := time.Now()
		if err := generator.GenerateAll(ctx, config); err != nil {
			log.Fatalf("Failed to generate test data: %v", err)
		}

		duration := time.Since(startTime)
		fmt.Printf("\n✅ Test data generation completed in %v\n", duration)
		fmt.Println()
		return
	}

	// Check if data exists
	var userCount int64
	db.Table("users").Count(&userCount)
	if userCount == 0 {
		fmt.Println("⚠️  No test data found!")
		fmt.Println("   Run: go run cmd/quick-test/main.go generate")
		fmt.Println()
		return
	}

	fmt.Printf("📊 Found %d users in database\n", userCount)
	fmt.Println()

	// Initialize repositories
	mysqlRepo := repository.NewMySQLPermissionRepository(db)
	zanzibarRepo := repository.NewZanzibarPermissionRepository(db)

	// Run quick benchmark
	fmt.Println("⚡ Running quick benchmark...")
	fmt.Println()

	config := service.DefaultBenchmarkConfig()
	config.WarmupRounds = 10
	config.TestRounds = 50
	config.Concurrency = 2
	config.OutputDir = "./benchmark-results"

	benchmarkSuite := service.NewBenchmarkSuite(db, mysqlRepo, zanzibarRepo)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	startTime := time.Now()
	if err := benchmarkSuite.RunAllBenchmarks(ctx, config); err != nil {
		log.Fatalf("Benchmark failed: %v", err)
	}

	duration := time.Since(startTime)

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║              Quick Benchmark Complete!                    ║")
	fmt.Printf("║               Total Duration: %23v              ║\n", duration)
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("📁 Results saved to:", config.OutputDir)
}
