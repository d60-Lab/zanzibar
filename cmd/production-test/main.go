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
	fmt.Println("║       Production Scale Test: Real-World Scenario         ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Production configuration
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

	// Set connection pool for production scale
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("✅ Database connection established")
	fmt.Println()

	// Check if we need to generate data
	if len(os.Args) > 1 && os.Args[1] == "generate" {
		fmt.Println("🎲 Generating PRODUCTION SCALE test dataset...")
		fmt.Println()

		generator := service.NewTestDataGenerator(db)

		// PRODUCTION configuration matching real business scenario
		config := service.GenerateConfig{
			NumUsers:          5000,         // Production: 5K employees
			NumDepartments:    500,          // Production: 500 departments (more granular)
			NumCustomers:      50000,        // Production: 50K customers
			NumDocuments:      100000,       // Production: 100K documents
			MaxDeptLevels:     5,
			MaxDeptMembers:    100,          // Larger departments
			MaxCustomerFollowers: 20,        // More followers per customer
			BatchSize:         5000,         // Larger batches for efficiency
		}

		fmt.Printf("Configuration:\n")
		fmt.Printf("  Users:     %d (employees)\n", config.NumUsers)
		fmt.Printf("  Depts:     %d (departments)\n", config.NumDepartments)
		fmt.Printf("  Customers: %d\n", config.NumCustomers)
		fmt.Printf("  Documents: %d\n", config.NumDocuments)
		fmt.Println()
		fmt.Printf("Expected storage:\n")
		fmt.Printf("  - MySQL permissions: ~2-5M rows (expanded)\n")
		fmt.Printf("  - Zanzibar tuples: ~400-600K tuples (compact)\n")
		fmt.Println()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		defer cancel()

		startTime := time.Now()
		if err := generator.GenerateAll(ctx, config); err != nil {
			log.Fatalf("Failed to generate test data: %v", err)
		}
		duration := time.Since(startTime)

		fmt.Println()
		fmt.Printf("✅ Test data generation completed in %v\n", duration)
		return
	}

	// Run benchmarks
	if len(os.Args) > 1 && os.Args[1] == "benchmark" {
		fmt.Println("⚡ Running production-scale benchmark...")
		fmt.Println()

		// Get user count for verification
		var userCount int64
		db.Raw("SELECT count(*) FROM `users`").Scan(&userCount)
		fmt.Printf("📊 Found %d users in database\n", userCount)
		fmt.Println()

		if userCount == 0 {
			log.Fatal("No users found. Please run 'go run cmd/production-test/main.go generate' first.")
		}

		mysqlRepo := repository.NewMySQLPermissionRepository(db)
		zanzibarRepo := repository.NewZanzibarPermissionRepository(db)

		config := service.DefaultBenchmarkConfig()
		config.WarmupRounds = 10
		config.TestRounds = 50
		config.Concurrency = 4  // Higher concurrency for production scale
		config.OutputDir = "./benchmark-results-production"

		benchmarkSuite := service.NewBenchmarkSuite(db, mysqlRepo, zanzibarRepo)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		startTime := time.Now()
		if err := benchmarkSuite.RunAllBenchmarks(ctx, config); err != nil {
			log.Fatalf("Benchmark failed: %v", err)
		}
		duration := time.Since(startTime)

		fmt.Println()
		fmt.Printf("╔════════════════════════════════════════════════════════════╗\n")
		fmt.Printf("║         Production Benchmark Complete!                  ║\n")
		fmt.Printf("║               Total Duration: %v              ║\n", duration)
		fmt.Printf("╚════════════════════════════════════════════════════════════╝\n")
		fmt.Println()
		fmt.Println("📁 Results saved to:", config.OutputDir)
		return
	}

	fmt.Println("Usage:")
	fmt.Println("  go run cmd/production-test/main.go generate  - Generate production scale test data")
	fmt.Println("  go run cmd/production-test/main.go benchmark  - Run production scale benchmarks")
}
