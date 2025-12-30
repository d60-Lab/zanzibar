package main

import (
	"context"
	"fmt"
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/d60-Lab/gin-template/internal/model"
	"github.com/d60-Lab/gin-template/internal/repository"
)

func main() {
	dsn := "root:123456@tcp(127.0.0.1:3306)/zanzibar_permission?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	mysqlRepo := repository.NewMySQLPermissionRepository(db)
	zanzibarRepo := repository.NewZanzibarPermissionRepository(db)

	var users []model.User
	db.Limit(20).Find(&users)

	var docs []model.Document
	db.Limit(20).Find(&docs)

	fmt.Println("🔍 Verifying MySQL vs Zanzibar consistency...")
	fmt.Println()

	inconsistencies := 0
	totalChecks := 0
	mysqlTrue := 0
	zanzibarTrue := 0
	bothTrue := 0

	for _, user := range users {
		for _, doc := range docs {
			totalChecks++

			mysqlResult, _ := mysqlRepo.CheckPermission(ctx, user.ID, doc.ID, "viewer")
			zanzibarResult, _ := zanzibarRepo.CheckPermission(ctx, user.ID, doc.ID, "viewer")

			mysqlHas := mysqlResult.HasPermission
			zanzibarHas := zanzibarResult.HasPermission

			if mysqlHas {
				mysqlTrue++
			}
			if zanzibarHas {
				zanzibarTrue++
			}
			if mysqlHas && zanzibarHas {
				bothTrue++
			}

			if mysqlHas != zanzibarHas {
				inconsistencies++
				fmt.Printf("❌ INCONSISTENCY: user=%s, doc=%s\n", user.ID, doc.ID)
				fmt.Printf("   MySQL: has=%v, sources=%v\n", mysqlHas, mysqlResult.Sources)
				fmt.Printf("   Zanzibar: has=%v, sources=%v\n", zanzibarHas, zanzibarResult.Sources)
				fmt.Println()
			}
		}
	}

	fmt.Printf("\n📊 Results:\n")
	fmt.Printf("   Total checks: %d\n", totalChecks)
	fmt.Printf("   MySQL permissions: %d\n", mysqlTrue)
	fmt.Printf("   Zanzibar permissions: %d\n", zanzibarTrue)
	fmt.Printf("   Both said yes: %d\n", bothTrue)
	fmt.Printf("   Inconsistencies: %d\n", inconsistencies)

	if inconsistencies == 0 {
		fmt.Printf("   ✅ All results are consistent!\n")
	} else {
		fmt.Printf("   ⚠️  Consistency rate: %.2f%%\n", float64(totalChecks-inconsistencies)*100/float64(totalChecks))
	}
}
