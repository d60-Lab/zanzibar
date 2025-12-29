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

	"github.com/maynardzanzibar/internal/repository"
	"github.com/maynardzanzibar/internal/service"
)

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║  Zanzibar vs MySQL Permission System Benchmark Suite       ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	 // Get database connection from environment or use default
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		dsn = "root:password@tcp(localhost:3306)/gin_template?charset=utf8mb4&parseTime=True&loc=Local"
	}

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

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("✅ Database connection established")
	fmt.Println()

	// Initialize repositories
	mysqlRepo := repository.NewMySQLPermissionRepository(db)
	zanzibarRepo := repository.NewZanzibarPermissionRepository(db)

	// Create benchmark suite
	benchmarkSuite := service.NewBenchmarkSuite(db, mysqlRepo, zanzibarRepo)

	// Check if we need to generate test data first
	if len(os.Args) > 1 && os.Args[1] == "generate" {
		fmt.Println("🎲 Generating test data...")
		fmt.Println("⚠️  This may take several hours for full dataset!")
		fmt.Println()

		generator := service.NewTestDataGenerator(db)
		config := service.DefaultConfig()

		// Ask for confirmation
		fmt.Print("Continue with full data generation? (y/N): ")
		var confirm string
		fmt.Scanln(&confirm)

		if confirm != "y" && confirm != "Y" {
			fmt.Println("❌ Aborted")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		defer cancel()

		if err := generator.GenerateAll(ctx, config); err != nil {
			log.Fatalf("Failed to generate test data: %v", err)
		}

		fmt.Println()
		fmt.Println("✅ Test data generation completed!")
		fmt.Println()
	}

	// Check if data exists
	var userCount int64
	db.Table("users").Count(&userCount)
	if userCount == 0 {
		fmt.Println("⚠️  No test data found!")
		fmt.Println("   Run: go run cmd/benchmark/main.go generate")
		fmt.Println()
		return
	}

	fmt.Printf("📊 Found %d users in database\n", userCount)
	fmt.Println()

	// Run benchmarks
	config := service.DefaultBenchmarkConfig()

	// Allow custom configuration
	if len(os.Args) > 1 && os.Args[1] == "quick" {
		fmt.Println("⚡ Running quick benchmark (reduced iterations)...")
		config.WarmupRounds = 10
		config.TestRounds = 100
		config.Concurrency = 2
	} else if len(os.Args) > 1 && os.Args[1] == "full" {
		fmt.Println("🚀 Running full benchmark suite...")
		config.WarmupRounds = 100
		config.TestRounds = 1000
		config.Concurrency = 10
	} else {
		fmt.Println("📊 Running standard benchmark...")
		fmt.Println("   Usage: go run cmd/benchmark/main.go [generate|quick|full]")
		fmt.Println()
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	startTime := time.Now()
	if err := benchmarkSuite.RunAllBenchmarks(ctx, config); err != nil {
		log.Fatalf("Benchmark failed: %v", err)
	}

	duration := time.Since(startTime)

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    Benchmark Complete!                      ║")
	fmt.Printf("║               Total Duration: %23v              ║\n", duration)
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("📁 Results saved to:", config.OutputDir)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("   1. Review CSV/JSON files for detailed data")
	fmt.Println("   2. Check summary report for key findings")
	fmt.Println("   3. Compare MySQL vs Zanzibar performance")
	fmt.Println("   4. Analyze maintenance operation improvements")
}
