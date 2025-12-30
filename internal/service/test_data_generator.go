package service

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/d60-Lab/gin-template/internal/model"
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

	// Phase 9: Generate document read status
	fmt.Println("\n📖 Phase 9: Generating document read status...")
	if err := g.generateDocumentReads(ctx); err != nil {
		return fmt.Errorf("failed to generate document reads: %w", err)
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
				var level4 *model.Department
				if g.r.Float64() < 0.3 && deptCount < config.NumDepartments {
					deptCount++
					level4 = &model.Department{
						ID:       fmt.Sprintf("dept-l4-%d-%d-%d-1", i, j, k),
						Name:     fmt.Sprintf("Team %c%d-%d-1", 'A'+i, j+1, k+1),
						ParentID: &level3.ID,
						Level:    4,
					}
					departments = append(departments, *level4)
				}

				// Level 5: Sub-teams (even smaller portion)
				if g.r.Float64() < 0.1 && deptCount < config.NumDepartments && level4 != nil {
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
		assignedDepts := make(map[string]bool) // Track assigned departments to avoid duplicates

		for d := 0; d < numDepartments; d++ {
			// Pick random department (avoid duplicates)
			var dept model.Department
			var attempts int
			for attempts = 0; attempts < 100; attempts++ {
				dept = departments[g.r.Intn(len(departments))]
				if !assignedDepts[dept.ID] {
					assignedDepts[dept.ID] = true
					break
				}
			}

			if attempts >= 100 {
				// Couldn't find unique department, skip this assignment
				continue
			}

			userDeptIDs = append(userDeptIDs, dept.ID)

			userDepts = append(userDepts, model.UserDepartment{
				UserID:       users[i].ID,
				DepartmentID: dept.ID,
				Role:         g.getRandomRole(),
				IsPrimary:    d == 0,
			})
		}

		if len(userDeptIDs) > 0 {
			users[i].PrimaryDepartmentID = &userDeptIDs[0]
		}

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

	// Get all departments (except level 5 which are sub-teams)
	var departments []model.Department
	if err := g.db.WithContext(ctx).Where("level < ?", 5).Find(&departments).Error; err != nil {
		return err
	}

	// Build department map for parent lookups
	deptMap := make(map[string]*model.Department)
	for i := range departments {
		deptMap[departments[i].ID] = &departments[i]
	}

	// Get ALL user-department relationships in ONE query
	var allUserDepts []model.UserDepartment
	if err := g.db.WithContext(ctx).Find(&allUserDepts).Error; err != nil {
		return err
	}

	// Group user-departments by department ID
	deptUserMap := make(map[string][]model.UserDepartment)
	for _, ud := range allUserDepts {
		deptUserMap[ud.DepartmentID] = append(deptUserMap[ud.DepartmentID], ud)
	}

	// Use a map to deduplicate management relations
	// Key: "managerID|subordinateID|departmentID" (using | as delimiter since IDs contain -)
	// Value: the lowest management level for this relation
	relationsMap := make(map[string]int) // key -> min level
	deptManagerUpdates := make(map[string]string) // dept ID -> manager ID

	// Assign managers to departments and build relations
	for _, dept := range departments {
		userDepts := deptUserMap[dept.ID]
		if len(userDepts) == 0 {
			continue
		}

		// Pick a manager from this department's users
		managerUserDept := userDepts[g.r.Intn(len(userDepts))]
		managerID := managerUserDept.UserID
		deptManagerUpdates[dept.ID] = managerID

		// Create management relations for all non-manager users
		for _, ud := range userDepts {
			if ud.UserID != managerID {
				// Direct manager relation (level 1)
				key := fmt.Sprintf("%s|%s|%s", managerID, ud.UserID, dept.ID)
				if currentLevel, exists := relationsMap[key]; !exists || currentLevel > 1 {
					relationsMap[key] = 1
				}

				// Build manager chain (levels 2-5) using deptMap
				currentDept := &dept
				currentLevel := 2

				for currentLevel <= 5 && currentDept.ParentID != nil {
					parentDept, exists := deptMap[*currentDept.ParentID]
					if !exists {
						break
					}

					// Get parent's manager if assigned
					if parentManagerID, hasManager := deptManagerUpdates[parentDept.ID]; hasManager {
						key := fmt.Sprintf("%s|%s|%s", parentManagerID, ud.UserID, dept.ID)
						if existingLevel, exists := relationsMap[key]; !exists || existingLevel > currentLevel {
							relationsMap[key] = currentLevel
						}
					}

					currentDept = parentDept
					currentLevel++
				}
			}
		}
	}

	// Convert map to slice
	managementRelations := make([]model.ManagementRelation, 0, len(relationsMap))
	for key, level := range relationsMap {
		// Parse key "managerID|subordinateID|departmentID"
		parts := strings.Split(key, "|")
		if len(parts) != 3 {
			continue // Skip malformed keys
		}

		managementRelations = append(managementRelations, model.ManagementRelation{
			ManagerUserID:     parts[0],
			SubordinateUserID: parts[1],
			DepartmentID:      parts[2],
			ManagementLevel:   level,
		})
	}

	// Batch update all department managers in ONE query
	if len(deptManagerUpdates) > 0 {
		sql := "UPDATE departments SET manager_id = CASE id "
		for deptID, managerID := range deptManagerUpdates {
			sql += fmt.Sprintf(" WHEN '%s' THEN '%s'", deptID, managerID)
		}
		sql += " END WHERE id IN ("
		first := true
		for deptID := range deptManagerUpdates {
			if !first {
				sql += ","
			}
			sql += fmt.Sprintf("'%s'", deptID)
			first = false
		}
		sql += ")"

		if err := g.db.WithContext(ctx).Exec(sql).Error; err != nil {
			return fmt.Errorf("failed to update department managers: %w", err)
		}
	}

	// Batch insert all management relations
	if len(managementRelations) > 0 {
		if err := g.db.WithContext(ctx).CreateInBatches(managementRelations, 1000).Error; err != nil {
			return err
		}
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

	// Assign customer followers using Zipfian distribution
	// This simulates real-world scenario:
	// - 10% large customers: 50-100 followers
	// - 30% medium customers: 10-30 followers
	// - 60% small customers: 1-5 followers
	// Global map to track all (customer_id, user_id) pairs to prevent duplicates
	customerFollowerKeys := make(map[string]bool)

	for idx, customer := range customers {
		var numFollowers int

		// Zipfian distribution based on customer index
		// Lower index = larger customer = more followers
		percentile := float64(idx) / float64(len(customers))

		if percentile < 0.1 {
			// Top 10% large customers: 50-100 followers
			numFollowers = 50 + g.r.Intn(51)
		} else if percentile < 0.4 {
			// Next 30% medium customers: 10-30 followers
			numFollowers = 10 + g.r.Intn(21)
		} else {
			// Remaining 60% small customers: 1-5 followers
			numFollowers = 1 + g.r.Intn(5)
		}

		for i := 0; i < numFollowers; i++ {
			// Try to find a unique user for this customer
			var attempts int
			var user model.User
			key := ""

			for attempts = 0; attempts < 100; attempts++ {
				user = users[g.r.Intn(len(users))]
				key = fmt.Sprintf("%s|%s", customer.ID, user.ID)
				if !customerFollowerKeys[key] {
					customerFollowerKeys[key] = true
					break
				}
			}

			if attempts >= 100 {
				// Couldn't find unique user, skip this assignment
				continue
			}

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

// generateMySQLPermissions builds expanded permission table (OPTIMIZED!)
// Uses pre-loaded data to avoid N+1 queries
func (g *TestDataGenerator) generateMySQLPermissions(ctx context.Context) error {
	fmt.Println("   Building MySQL expanded permissions (optimized version)...")

	startTime := time.Now()

	// Step 1: Pre-load ALL required data in bulk (avoid N+1 queries)
	fmt.Println("   📥 Pre-loading data...")

	// Load all documents
	var documents []model.Document
	if err := g.db.WithContext(ctx).Find(&documents).Error; err != nil {
		return err
	}
	fmt.Printf("      - Loaded %d documents\n", len(documents))

	// Load all customer followers and build lookup map: customerID -> []userID
	var allFollowers []model.CustomerFollower
	if err := g.db.WithContext(ctx).Find(&allFollowers).Error; err != nil {
		return err
	}
	customerFollowersMap := make(map[string][]string)
	for _, f := range allFollowers {
		customerFollowersMap[f.CustomerID] = append(customerFollowersMap[f.CustomerID], f.UserID)
	}
	fmt.Printf("      - Loaded %d customer followers\n", len(allFollowers))

	// Load all management relations and build lookup map: subordinateUserID -> []managerUserID
	var allManagers []model.ManagementRelation
	if err := g.db.WithContext(ctx).Find(&allManagers).Error; err != nil {
		return err
	}
	managerMap := make(map[string][]string)
	for _, m := range allManagers {
		managerMap[m.SubordinateUserID] = append(managerMap[m.SubordinateUserID], m.ManagerUserID)
	}
	fmt.Printf("      - Loaded %d management relations\n", len(allManagers))

	// Load all superusers (once!)
	var superusers []model.User
	if err := g.db.WithContext(ctx).Where("is_superuser = ?", true).Find(&superusers).Error; err != nil {
		return err
	}
	superuserIDs := make([]string, len(superusers))
	for i, u := range superusers {
		superuserIDs[i] = u.ID
	}
	fmt.Printf("      - Loaded %d superusers\n", len(superusers))

	// Step 2: Generate permissions using in-memory lookups (FAST!)
	fmt.Println("   🔄 Generating permissions...")

	permissions := make([]model.DocumentPermissionMySQL, 0, 10000000)
	permissionKeys := make(map[string]bool)
	processedDocs := 0
	totalPermissions := 0

	addPermission := func(perm model.DocumentPermissionMySQL) {
		key := fmt.Sprintf("%s|%s|%s", perm.UserID, perm.DocumentID, perm.PermissionType)
		if !permissionKeys[key] {
			permissionKeys[key] = true
			permissions = append(permissions, perm)
		}
	}

	for _, doc := range documents {
		processedDocs++

		// 1. Creator gets owner permission
		addPermission(model.DocumentPermissionMySQL{
			UserID:         doc.CreatorID,
			DocumentID:     doc.ID,
			PermissionType: "owner",
			SourceType:     "direct",
			SourceID:       &doc.ID,
		})

		// 2. Creator gets viewer permission
		addPermission(model.DocumentPermissionMySQL{
			UserID:         doc.CreatorID,
			DocumentID:     doc.ID,
			PermissionType: "viewer",
			SourceType:     "direct",
			SourceID:       &doc.ID,
		})

		// 3. Customer followers get viewer permission (from pre-loaded map)
		if followers, ok := customerFollowersMap[doc.CustomerID]; ok {
			for _, followerID := range followers {
				addPermission(model.DocumentPermissionMySQL{
					UserID:         followerID,
					DocumentID:     doc.ID,
					PermissionType: "viewer",
					SourceType:     "customer_follower",
					SourceID:       &doc.CustomerID,
				})
			}
		}

		// 4. Manager chain gets viewer permission (from pre-loaded map)
		if managers, ok := managerMap[doc.CreatorID]; ok {
			// Deduplicate managers
			seen := make(map[string]bool)
			for _, managerID := range managers {
				if !seen[managerID] {
					seen[managerID] = true
					addPermission(model.DocumentPermissionMySQL{
						UserID:         managerID,
						DocumentID:     doc.ID,
						PermissionType: "viewer",
						SourceType:     "manager_chain",
						SourceID:       &doc.CreatorID,
					})
				}
			}
		}

		// 5. Superusers get all permissions (from pre-loaded list)
		for _, superuserID := range superuserIDs {
			addPermission(model.DocumentPermissionMySQL{
				UserID:         superuserID,
				DocumentID:     doc.ID,
				PermissionType: "viewer",
				SourceType:     "superuser",
			})
		}

		// Batch insert every 100000 permissions to avoid memory issues
		if len(permissions) >= 100000 {
			if err := g.db.WithContext(ctx).CreateInBatches(permissions, 5000).Error; err != nil {
				return fmt.Errorf("failed to insert permissions batch: %w", err)
			}
			totalPermissions += len(permissions)
			fmt.Printf("      Processed %d/%d documents, %d permissions inserted...\n", processedDocs, len(documents), totalPermissions)
			permissions = permissions[:0] // Reuse slice
			permissionKeys = make(map[string]bool) // Reset dedup map for memory
		}
	}

	// Insert remaining permissions
	if len(permissions) > 0 {
		if err := g.db.WithContext(ctx).CreateInBatches(permissions, 5000).Error; err != nil {
			return err
		}
		totalPermissions += len(permissions)
	}

	duration := time.Since(startTime)
	fmt.Printf("   ✅ Created %d MySQL permissions in %v\n", totalPermissions, duration)

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

// generateDocumentReads generates document read status for users
// Only marks documents that users actually have permission to access
// Simulates that users have read 60% of their accessible documents
// OPTIMIZED: Uses bulk query instead of per-user queries
func (g *TestDataGenerator) generateDocumentReads(ctx context.Context) error {
	fmt.Println("   Marking documents as read (optimized)...")

	startTime := time.Now()

	// Step 1: Get all user-document permission pairs in ONE query
	fmt.Println("      📥 Loading user-document permissions...")
	type UserDocPair struct {
		UserID     string
		DocumentID string
	}
	var allPermissions []UserDocPair
	if err := g.db.WithContext(ctx).
		Table("document_permissions_mysql").
		Select("DISTINCT user_id, document_id").
		Where("permission_type = ?", "viewer").
		Find(&allPermissions).Error; err != nil {
		return err
	}
	fmt.Printf("      - Loaded %d user-document pairs\n", len(allPermissions))

	// Step 2: Get superuser IDs to exclude
	var superuserIDs []string
	if err := g.db.WithContext(ctx).
		Table("users").
		Where("is_superuser = ?", true).
		Pluck("id", &superuserIDs).Error; err != nil {
		return err
	}
	superuserSet := make(map[string]bool)
	for _, id := range superuserIDs {
		superuserSet[id] = true
	}

	// Step 3: Generate read records (60% of accessible docs)
	fmt.Println("      🔄 Generating read records...")
	documentReads := make([]model.DocumentRead, 0, len(allPermissions)/2)
	readKeys := make(map[string]bool)
	now := time.Now()

	for _, perm := range allPermissions {
		// Skip superusers
		if superuserSet[perm.UserID] {
			continue
		}

		// 60% chance to mark as read
		if g.r.Float64() < 0.6 {
			key := fmt.Sprintf("%s|%s", perm.UserID, perm.DocumentID)
			if !readKeys[key] {
				readKeys[key] = true
				documentReads = append(documentReads, model.DocumentRead{
					UserID:     perm.UserID,
					DocumentID: perm.DocumentID,
					ReadAt:     now,
				})
			}
		}
	}

	// Step 4: Batch insert
	fmt.Printf("      💾 Inserting %d read records...\n", len(documentReads))
	if len(documentReads) > 0 {
		if err := g.db.WithContext(ctx).CreateInBatches(documentReads, 5000).Error; err != nil {
			return err
		}
	}

	duration := time.Since(startTime)
	fmt.Printf("   ✅ Created %d document read records in %v\n", len(documentReads), duration)
	return nil
}
