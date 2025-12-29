package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"gorm.io/gorm"

	"github.com/maynardzanzibar/internal/model"
)

// TestDataGenerator generates realistic test data for permission comparison
type TestDataGenerator struct {
	db *gorm.DB
	r  *rand.Rand
}

// NewTestDataGenerator creates a new test data generator
func NewTestDataGenerator(db *gorm.DB) *TestDataGenerator {
	return &TestDataGenerator{
		db: db,
		r:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateConfig holds configuration for data generation
type GenerateConfig struct {
	NumUsers          int
	NumDepartments    int
	NumCustomers      int
	NumDocuments      int
	MaxDeptLevels     int
	MaxDeptMembers    int
	MaxCustomerFollowers int
	BatchSize         int
}

// DefaultConfig returns default generation configuration
func DefaultConfig() GenerateConfig {
	return GenerateConfig{
		NumUsers:          10000,
		NumDepartments:    2000,
		NumCustomers:      100000,
		NumDocuments:      500000,
		MaxDeptLevels:     5,
		MaxDeptMembers:    50,
		MaxCustomerFollowers: 10,
		BatchSize:         1000,
	}
}

// GenerateAll generates all test data
func (g *TestDataGenerator) GenerateAll(ctx context.Context, config GenerateConfig) error {
	fmt.Println("🚀 Starting test data generation...")
	startTime := time.Now()

	// Phase 1: Build organization hierarchy
	fmt.Println("\n📊 Phase 1: Building organization hierarchy...")
	if err := g.generateDepartmentHierarchy(ctx, config); err != nil {
		return fmt.Errorf("failed to generate departments: %w", err)
	}

	// Phase 2: Generate users
	fmt.Println("\n👥 Phase 2: Generating users...")
	if err := g.generateUsers(ctx, config); err != nil {
		return fmt.Errorf("failed to generate users: %w", err)
	}

	// Phase 3: Build management relations
	fmt.Println("\n🔗 Phase 3: Building management relations...")
	if err := g.generateManagementRelations(ctx); err != nil {
		return fmt.Errorf("failed to generate management relations: %w", err)
	}

	// Phase 4: Generate customers
	fmt.Println("\n🏢 Phase 4: Generating customers...")
	if err := g.generateCustomers(ctx, config); err != nil {
		return fmt.Errorf("failed to generate customers: %w", err)
	}

	// Phase 5: Generate documents
	fmt.Println("\n📄 Phase 5: Generating documents...")
	if err := g.generateDocuments(ctx, config); err != nil {
		return fmt.Errorf("failed to generate documents: %w", err)
	}

	// Phase 6: Build MySQL permissions (EXPENSIVE)
	fmt.Println("\n🗄️  Phase 6: Building MySQL expanded permissions...")
	if err := g.generateMySQLPermissions(ctx); err != nil {
		return fmt.Errorf("failed to generate MySQL permissions: %w", err)
	}

	// Phase 7: Build Zanzibar tuples
	fmt.Println("\n🔗 Phase 7: Building Zanzibar tuples...")
	if err := g.generateZanzibarTuples(ctx); err != nil {
		return fmt.Errorf("failed to generate Zanzibar tuples: %w", err)
	}

	// Phase 8: Add superusers
	fmt.Println("\n👑 Phase 8: Adding superusers...")
	if err := g.generateSuperusers(ctx); err != nil {
		return fmt.Errorf("failed to generate superusers: %w", err)
	}

	duration := time.Since(startTime)
	fmt.Printf("\n✅ Test data generation completed in %v\n", duration)

	// Print statistics
	if err := g.printStatistics(ctx); err != nil {
		return err
	}

	return nil
}

// generateDepartmentHierarchy builds a 5-level department tree
func (g *TestDataGenerator) generateDepartmentHierarchy(ctx context.Context, config GenerateConfig) error {
	fmt.Println("   Creating department hierarchy...")

	departments := make([]model.Department, 0, config.NumDepartments)
	deptCount := 0

	// Level 1: Headquarters (10 depts)
	level1Count := 10
	for i := 0; i < level1Count && deptCount < config.NumDepartments; i++ {
		deptCount++
		dept := model.Department{
			ID:       fmt.Sprintf("dept-l1-%d", i),
			Name:     fmt.Sprintf("Headquarters %c", 'A'+i),
			ParentID: nil,
			Level:    1,
		}
		departments = append(departments, dept)

		// Level 2: Business Units under each HQ (10 each)
		level2Count := 10
		for j := 0; j < level2Count && deptCount < config.NumDepartments; j++ {
			deptCount++
			level2 := model.Department{
				ID:       fmt.Sprintf("dept-l2-%d-%d", i, j),
				Name:     fmt.Sprintf("Business Unit %c%d", 'A'+i, j+1),
				ParentID: &dept.ID,
				Level:    2,
			}
			departments = append(departments, level2)

			// Level 3: Departments under each BU (20 each)
			level3Count := 20
			for k := 0; k < level3Count && deptCount < config.NumDepartments; k++ {
				deptCount++
				level3 := model.Department{
					ID:       fmt.Sprintf("dept-l3-%d-%d-%d", i, j, k),
					Name:     fmt.Sprintf("Department %c%d-%d", 'A'+i, j+1, k+1),
					ParentID: &level2.ID,
					Level:    3,
				}
				departments = append(departments, level3)

				// Level 4: Teams under each department (smaller portion)
				if g.r.Float64() < 0.3 && deptCount < config.NumDepartments {
					deptCount++
					level4 := model.Department{
						ID:       fmt.Sprintf("dept-l4-%d-%d-%d-1", i, j, k),
						Name:     fmt.Sprintf("Team %c%d-%d-1", 'A'+i, j+1, k+1),
						ParentID: &level3.ID,
						Level:    4,
					}
					departments = append(departments, level4)
				}

				// Level 5: Sub-teams (even smaller portion)
				if g.r.Float64() < 0.1 && deptCount < config.NumDepartments {
					deptCount++
					level5 := model.Department{
						ID:       fmt.Sprintf("dept-l5-%d-%d-%d-1-1", i, j, k),
						Name:     fmt.Sprintf("Sub-Team %c%d-%d-1-1", 'A'+i, j+1, k+1),
						ParentID: &level4.ID,
						Level:    5,
					}
					departments = append(departments, level5)
				}
			}
		}
	}

	// Batch insert departments
	if err := g.db.WithContext(ctx).CreateInBatches(departments, config.BatchSize).Error; err != nil {
		return err
	}

	fmt.Printf("   ✅ Created %d departments\n", len(departments))
	return nil
}

// generateUsers creates users with multi-department affiliation
func (g *TestDataGenerator) generateUsers(ctx context.Context, config GenerateConfig) error {
	fmt.Println("   Creating users...")

	users := make([]model.User, config.NumUsers)
	userDepts := make([]model.UserDepartment, 0, int(float64(config.NumUsers)*1.3)) // 1.3 depts per user avg

	// Get all departments
	var departments []model.Department
	if err := g.db.WithContext(ctx).Find(&departments).Error; err != nil {
		return err
	}

	// Create users
	for i := 0; i < config.NumUsers; i++ {
		users[i] = model.User{
			ID:    fmt.Sprintf("user-%d", i),
			Name:  fmt.Sprintf("User %d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
		}

		// Assign to departments (realistic distribution)
		numDepartments := g.getNumDepartmentsForUser()
		userDeptIDs := make([]string, 0, numDepartments)

		for d := 0; d < numDepartments; d++ {
			// Pick random department
			dept := departments[g.r.Intn(len(departments))]
			userDeptIDs = append(userDeptIDs, dept.ID)

			userDepts = append(userDepts, model.UserDepartment{
				UserID:       users[i].ID,
				DepartmentID: dept.ID,
				Role:         g.getRandomRole(),
				IsPrimary:    d == 0,
			})
		}

		users[i].PrimaryDepartmentID = &userDeptIDs[0]

		// Randomly assign as superuser (1% of users)
		if i < 10 {
			users[i].IsSuperuser = true
		}
	}

	// Batch insert users
	if err := g.db.WithContext(ctx).CreateInBatches(users, config.BatchSize).Error; err != nil {
		return err
	}

	// Batch insert user-department relationships
	if err := g.db.WithContext(ctx).CreateInBatches(userDepts, config.BatchSize).Error; err != nil {
		return err
	}

	fmt.Printf("   ✅ Created %d users with %d department affiliations\n", len(users), len(userDepts))
	return nil
}

// getNumDepartmentsForUser returns number of departments for a user (weighted distribution)
func (g *TestDataGenerator) getNumDepartmentsForUser() int {
	r := g.r.Float64()
	if r < 0.80 {
		return 1 // 80% of users: 1 department
	} else if r < 0.95 {
		return 2 // 15% of users: 2 departments
	} else if r < 0.99 {
		return 3 // 4% of users: 3 departments
	} else {
		return g.r.Intn(2) + 4 // 1% of users: 4-5 departments
	}
}

// getRandomRole returns a random department role
func (g *TestDataGenerator) getRandomRole() string {
	roles := []string{"member", "member", "member", "member", "leader", "director"} // Weighted towards members
	return roles[g.r.Intn(len(roles))]
}

// generateManagementRelations builds management paths
func (g *TestDataGenerator) generateManagementRelations(ctx context.Context) error {
	fmt.Println("   Building management relations...")

	// Get all departments
	var departments []model.Department
	if err := g.db.WithContext(ctx).Where("level < ?", 5).Find(&departments).Error; err != nil {
		return err
	}

	// Get all users
	var users []model.User
	if err := g.db.WithContext(ctx).Find(&users).Error; err != nil {
		return err
	}

	userMap := make(map[string]*model.User)
	for i := range users {
		userMap[users[i].ID] = &users[i]
	}

	managementRelations := make([]model.ManagementRelation, 0)

	// Assign managers to departments
	for _, dept := range departments {
		// Find users in this department
		var userDepts []model.UserDepartment
		if err := g.db.WithContext(ctx).
			Where("department_id = ?", dept.ID).
			Find(&userDepts).Error; err != nil {
			return err
		}

		if len(userDepts) == 0 {
			continue
		}

		// Pick a manager from this department's users
		managerUserDept := userDepts[g.r.Intn(len(userDepts))]
		managerID := managerUserDept.UserID

		// Update department with manager
		if err := g.db.WithContext(ctx).
			Model(&model.Department{}).
			Where("id = ?", dept.ID).
			Update("manager_id", managerID).Error; err != nil {
			return err
		}

		// Create management relations for all non-manager users
		for _, ud := range userDepts {
			if ud.UserID != managerID {
				// Calculate management level (1 for direct manager)
				level := 1
				managementRelations = append(managementRelations, model.ManagementRelation{
					ManagerUserID:     managerID,
					SubordinateUserID: ud.UserID,
					DepartmentID:      dept.ID,
					ManagementLevel:   level,
				})

				// Add higher-level management relations (2-5 levels up)
				// This represents the manager chain
				currentDept := &dept
				currentLevel := 2

				for currentLevel <= 5 && currentDept.ParentID != nil {
					// Find parent department
					var parentDept model.Department
					if err := g.db.WithContext(ctx).
						Where("id = ?", *currentDept.ParentID).
						First(&parentDept).Error; err != nil {
						break
					}

					if parentDept.ManagerID != nil {
						managementRelations = append(managementRelations, model.ManagementRelation{
							ManagerUserID:     *parentDept.ManagerID,
							SubordinateUserID: ud.UserID,
							DepartmentID:      dept.ID,
							ManagementLevel:   currentLevel,
						})
					}

					currentDept = &parentDept
					currentLevel++
				}
			}
		}
	}

	// Batch insert management relations
	if err := g.db.WithContext(ctx).CreateInBatches(managementRelations, 1000).Error; err != nil {
		return err
	}

	fmt.Printf("   ✅ Created %d management relations\n", len(managementRelations))
	return nil
}

// generateCustomers creates customers
func (g *TestDataGenerator) generateCustomers(ctx context.Context, config GenerateConfig) error {
	fmt.Println("   Creating customers...")

	customers := make([]model.Customer, config.NumCustomers)
	for i := 0; i < config.NumCustomers; i++ {
		customers[i] = model.Customer{
			ID:   fmt.Sprintf("customer-%d", i),
			Name: fmt.Sprintf("Customer %d", i),
		}
	}

	if err := g.db.WithContext(ctx).CreateInBatches(customers, config.BatchSize).Error; err != nil {
		return err
	}

	fmt.Printf("   ✅ Created %d customers\n", len(customers))
	return nil
}

// generateDocuments creates documents with Zipfian distribution
func (g *TestDataGenerator) generateDocuments(ctx context.Context, config GenerateConfig) error {
	fmt.Println("   Creating documents...")

	// Get all customers
	var customers []model.Customer
	if err := g.db.WithContext(ctx).Find(&customers).Error; err != nil {
		return err
	}

	// Get users for creators
	var users []model.User
	if err := g.db.WithContext(ctx).Limit(1000).Find(&users).Error; err != nil {
		return err
	}

	documents := make([]model.Document, 0, config.NumDocuments)
	customerFollowers := make([]model.CustomerFollower, 0)

	docCount := 0
	for i, customer := range customers {
		// Zipfian distribution: some customers have many documents
		numDocs := g.getNumDocsForCustomer(i, len(customers))

		for j := 0; j < numDocs && docCount < config.NumDocuments; j++ {
			docCount++
			doc := model.Document{
				ID:         fmt.Sprintf("doc-%d", docCount),
				Title:      fmt.Sprintf("Document %d for Customer %d", j+1, i),
				CustomerID: customer.ID,
				CreatorID:  users[g.r.Intn(len(users))].ID,
			}
			documents = append(documents, doc)
		}
	}

	// Assign customer followers (1-10 per customer)
	for _, customer := range customers {
		numFollowers := g.r.Intn(config.MaxCustomerFollowers) + 1
		for i := 0; i < numFollowers; i++ {
			user := users[g.r.Intn(len(users))]
			customerFollowers = append(customerFollowers, model.CustomerFollower{
				CustomerID: customer.ID,
				UserID:     user.ID,
			})
		}
	}

	// Batch insert documents
	if err := g.db.WithContext(ctx).CreateInBatches(documents, config.BatchSize).Error; err != nil {
		return err
	}

	// Batch insert customer followers
	if err := g.db.WithContext(ctx).CreateInBatches(customerFollowers, config.BatchSize).Error; err != nil {
		return err
	}

	fmt.Printf("   ✅ Created %d documents with %d customer followers\n", len(documents), len(customerFollowers))
	return nil
}

// getNumDocsForCustomer returns number of documents for a customer (Zipfian distribution)
func (g *TestDataGenerator) getNumDocsForCustomer(customerIndex, totalCustomers int) int {
	// Simple Zipf-like distribution
	// First customers get more documents
	rank := customerIndex + 1
	if rank == 1 {
		return g.r.Intn(400) + 100 // 100-500 docs
	} else if rank <= totalCustomers/100 {
		return g.r.Intn(100) + 20 // 20-120 docs (top 1%)
	} else if rank <= totalCustomers/10 {
		return g.r.Intn(30) + 5 // 5-35 docs (top 10%)
	} else if rank <= totalCustomers/3 {
		return g.r.Intn(10) + 1 // 1-11 docs (top 33%)
	} else {
		return g.r.Intn(5) + 1 // 1-6 docs (rest)
	}
}

// generateMySQLPermissions builds expanded permission table (EXPENSIVE!)
func (g *TestDataGenerator) generateMySQLPermissions(ctx context.Context) error {
	fmt.Println("   Building MySQL expanded permissions (this will take a while)...")

	startTime := time.Now()

	// Get all documents
	var documents []model.Document
	if err := g.db.WithContext(ctx).Find(&documents).Error; err != nil {
		return err
	}

	permissions := make([]model.DocumentPermissionMySQL, 0, 10000000) // Pre-allocate for 10M
	processedDocs := 0

	for _, doc := range documents {
		processedDocs++

		// 1. Creator gets owner permission
		permissions = append(permissions, model.DocumentPermissionMySQL{
			UserID:         doc.CreatorID,
			DocumentID:     doc.ID,
			PermissionType: "owner",
			SourceType:     "direct",
			SourceID:       &doc.ID,
		})

		// 2. Creator gets viewer permission
		permissions = append(permissions, model.DocumentPermissionMySQL{
			UserID:         doc.CreatorID,
			DocumentID:     doc.ID,
			PermissionType: "viewer",
			SourceType:     "direct",
			SourceID:       &doc.ID,
		})

		// 3. Customer followers get viewer permission
		var followers []model.CustomerFollower
		if err := g.db.WithContext(ctx).
			Where("customer_id = ?", doc.CustomerID).
			Find(&followers).Error; err != nil {
			return err
		}

		for _, follower := range followers {
			permissions = append(permissions, model.DocumentPermissionMySQL{
				UserID:         follower.UserID,
				DocumentID:     doc.ID,
				PermissionType: "viewer",
				SourceType:     "customer_follower",
				SourceID:       &doc.CustomerID,
			})
		}

		// 4. Manager chain gets viewer permission (EXPANSION)
		// Find all managers for the creator
		var managerRelations []model.ManagementRelation
		if err := g.db.WithContext(ctx).
			Where("subordinate_user_id = ?", doc.CreatorID).
			Find(&managerRelations).Error; err != nil {
			return err
		}

		// Collect unique manager IDs
		managerIDs := make(map[string]bool)
		for _, rel := range managerRelations {
			managerIDs[rel.ManagerUserID] = true
		}

		// Add permissions for all managers
		for managerID := range managerIDs {
			permissions = append(permissions, model.DocumentPermissionMySQL{
				UserID:         managerID,
				DocumentID:     doc.ID,
				PermissionType: "viewer",
				SourceType:     "manager_chain",
				SourceID:       &doc.CreatorID,
			})
		}

		// 5. Superusers get all permissions
		var superusers []model.User
		if err := g.db.WithContext(ctx).
			Where("is_superuser = ?", true).
			Find(&superusers).Error; err != nil {
			return err
		}

		for _, superuser := range superusers {
			permissions = append(permissions, model.DocumentPermissionMySQL{
				UserID:         superuser.ID,
				DocumentID:     doc.ID,
				PermissionType: "viewer",
				SourceType:     "superuser",
			})
		}

		// Batch insert every 10000 documents to avoid memory issues
		if len(permissions) >= 100000 {
			if err := g.db.WithContext(ctx).CreateInBatches(permissions, 1000).Error; err != nil {
				return fmt.Errorf("failed to insert permissions batch: %w", err)
			}
			fmt.Printf("   Processed %d/%d documents, %d permissions so far...\n", processedDocs, len(documents), len(permissions))
			permissions = make([]model.DocumentPermissionMySQL, 0, 100000)
		}
	}

	// Insert remaining permissions
	if len(permissions) > 0 {
		if err := g.db.WithContext(ctx).CreateInBatches(permissions, 1000).Error; err != nil {
			return err
		}
	}

	duration := time.Since(startTime)
	fmt.Printf("   ✅ Created MySQL permissions in %v\n", duration)

	return nil
}

// generateZanzibarTuples builds tuple-based permissions
func (g *TestDataGenerator) generateZanzibarTuples(ctx context.Context) error {
	fmt.Println("   Building Zanzibar tuples...")

	startTime := time.Now()
	tuples := make([]model.RelationTuple, 0, 1000000)

	// 1. Document creator permissions (direct)
	var documents []model.Document
	if err := g.db.WithContext(ctx).Find(&documents).Error; err != nil {
		return err
	}

	for _, doc := range documents {
		tuples = append(tuples, model.RelationTuple{
			Namespace:        "document",
			ObjectID:         doc.ID,
			Relation:         "owner",
			SubjectNamespace: "user",
			SubjectID:        doc.CreatorID,
		})

		tuples = append(tuples, model.RelationTuple{
			Namespace:        "document",
			ObjectID:         doc.ID,
			Relation:         "viewer",
			SubjectNamespace: "user",
			SubjectID:        doc.CreatorID,
		})

		tuples = append(tuples, model.RelationTuple{
			Namespace:        "document",
			ObjectID:         doc.ID,
			Relation:         "owner_customer",
			SubjectNamespace: "customer",
			SubjectID:        doc.CustomerID,
		})
	}

	// 2. Customer follower tuples
	var followers []model.CustomerFollower
	if err := g.db.WithContext(ctx).Find(&followers).Error; err != nil {
		return err
	}

	for _, follower := range followers {
		tuples = append(tuples, model.RelationTuple{
			Namespace:        "customer",
			ObjectID:         follower.CustomerID,
			Relation:         "follower",
			SubjectNamespace: "user",
			SubjectID:        follower.UserID,
		})
	}

	// 3. Department membership tuples
	var userDepts []model.UserDepartment
	if err := g.db.WithContext(ctx).Find(&userDepts).Error; err != nil {
		return err
	}

	for _, ud := range userDepts {
		tuples = append(tuples, model.RelationTuple{
			Namespace:        "department",
			ObjectID:         ud.DepartmentID,
			Relation:         "member",
			SubjectNamespace: "user",
			SubjectID:        ud.UserID,
		})
	}

	// 4. Department manager tuples
	var departments []model.Department
	if err := g.db.WithContext(ctx).Where("manager_id IS NOT NULL").Find(&departments).Error; err != nil {
		return err
	}

	for _, dept := range departments {
		if dept.ManagerID != nil {
			tuples = append(tuples, model.RelationTuple{
				Namespace:        "department",
				ObjectID:         dept.ID,
				Relation:         "manager",
				SubjectNamespace: "user",
				SubjectID:        *dept.ManagerID,
			})
		}
	}

	// Batch insert tuples
	if err := g.db.WithContext(ctx).CreateInBatches(tuples, 1000).Error; err != nil {
		return err
	}

	duration := time.Since(startTime)
	fmt.Printf("   ✅ Created %d Zanzibar tuples in %v\n", len(tuples), duration)

	return nil
}

// generateSuperusers adds superuser tuples
func (g *TestDataGenerator) generateSuperusers(ctx context.Context) error {
	var superusers []model.User
	if err := g.db.WithContext(ctx).Where("is_superuser = ?", true).Find(&superusers).Error; err != nil {
		return err
	}

	tuples := make([]model.RelationTuple, len(superusers))
	for i, user := range superusers {
		tuples[i] = model.RelationTuple{
			Namespace:        "system",
			ObjectID:         "root",
			Relation:         "admin",
			SubjectNamespace: "user",
			SubjectID:        user.ID,
		}
	}

	if err := g.db.WithContext(ctx).Create(&tuples).Error; err != nil {
		return err
	}

	fmt.Printf("   ✅ Added %d superuser tuples\n", len(tuples))
	return nil
}

// printStatistics prints generation statistics
func (g *TestDataGenerator) printStatistics(ctx context.Context) error {
	fmt.Println("\n📊 Generation Statistics:")

	// Count entities
	var userCount, deptCount, customerCount, docCount, mysqlPermCount, zanzibarTupleCount int64

	g.db.WithContext(ctx).Model(&model.User{}).Count(&userCount)
	g.db.WithContext(ctx).Model(&model.Department{}).Count(&deptCount)
	g.db.WithContext(ctx).Model(&model.Customer{}).Count(&customerCount)
	g.db.WithContext(ctx).Model(&model.Document{}).Count(&docCount)
	g.db.WithContext(ctx).Model(&model.DocumentPermissionMySQL{}).Count(&mysqlPermCount)
	g.db.WithContext(ctx).Model(&model.RelationTuple{}).Count(&zanzibarTupleCount)

	fmt.Printf("   Users:           %d\n", userCount)
	fmt.Printf("   Departments:     %d\n", deptCount)
	fmt.Printf("   Customers:       %d\n", customerCount)
	fmt.Printf("   Documents:       %d\n", docCount)
	fmt.Printf("   MySQL Perms:     %d\n", mysqlPermCount)
	fmt.Printf("   Zanzibar Tuples: %d\n", zanzibarTupleCount)

	// Storage comparison
	fmt.Println("\n💾 Storage Comparison:")

	var mysqlStats, zanzibarStats model.StorageStats

	g.db.WithContext(ctx).Raw(`
		SELECT
			'MySQL' as engine_type,
			'document_permissions_mysql' as table_name,
			TABLE_ROWS as row_count,
			ROUND(DATA_LENGTH / 1024 / 1024, 2) as data_size_mb,
			ROUND(INDEX_LENGTH / 1024 / 1024, 2) as index_size_mb,
			ROUND((DATA_LENGTH + INDEX_LENGTH) / 1024 / 1024, 2) as total_size_mb
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'document_permissions_mysql'
	`).Scan(&mysqlStats)

	g.db.WithContext(ctx).Raw(`
		SELECT
			'Zanzibar' as engine_type,
			'relation_tuples' as table_name,
			TABLE_ROWS as row_count,
			ROUND(DATA_LENGTH / 1024 / 1024, 2) as data_size_mb,
			ROUND(INDEX_LENGTH / 1024 / 1024, 2) as index_size_mb,
			ROUND((DATA_LENGTH + INDEX_LENGTH) / 1024 / 1024, 2) as total_size_mb
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'relation_tuples'
	`).Scan(&zanzibarStats)

	fmt.Printf("   MySQL:   %d rows, %.2f MB\n", mysqlStats.RowCount, mysqlStats.TotalSizeMB)
	fmt.Printf("   Zanzibar: %d rows, %.2f MB\n", zanzibarStats.RowCount, zanzibarStats.TotalSizeMB)

	if mysqlStats.RowCount > 0 {
		reduction := float64(mysqlStats.RowCount-zanzibarStats.RowCount) / float64(mysqlStats.RowCount) * 100
		fmt.Printf("   Reduction: %.1f%% fewer rows\n", reduction)
	}

	return nil
}
