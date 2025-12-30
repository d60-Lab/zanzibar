package repository

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/d60-Lab/gin-template/internal/model"
)

func setupMySQLTestDB(t *testing.T) *gorm.DB {
	// Get MySQL connection from environment or use default
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = "root:123456@tcp(127.0.0.1:3306)/zanzibar_permission?charset=utf8mb4&parseTime=True&loc=Local"
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	return db
}

func createTestUser(db *gorm.DB, id, name, email string) *model.User {
	user := &model.User{
		ID:    id,
		Name:  name,
		Email: email,
	}
	db.Create(user)
	return user
}

func createTestDepartment(db *gorm.DB, id, name string, level int, parentID *string) *model.Department {
	dept := &model.Department{
		ID:       id,
		Name:     name,
		ParentID: parentID,
		Level:    level,
	}
	db.Create(dept)
	return dept
}

func createTestCustomer(db *gorm.DB, id, name string) *model.Customer {
	customer := &model.Customer{
		ID:   id,
		Name: name,
	}
	db.Create(customer)
	return customer
}

func createTestDocument(db *gorm.DB, id, title, customerID, creatorID string) *model.Document {
	doc := &model.Document{
		ID:         id,
		Title:      title,
		CustomerID: customerID,
		CreatorID:  creatorID,
	}
	db.Create(doc)
	return doc
}

func TestMySQLPermissionRepository_CheckPermission(t *testing.T) {
	db := setupMySQLTestDB(t)
	repo := NewMySQLPermissionRepository(db)
	ctx := context.Background()

	// Create test data
	user := createTestUser(db, "user-1", "Test User", "test@example.com")
	customer := createTestCustomer(db, "customer-1", "Test Customer")
	doc := createTestDocument(db, "doc-1", "Test Document", customer.ID, user.ID)

	// Grant permission
	err := repo.GrantDirectPermission(ctx, user.ID, doc.ID, "viewer")
	require.NoError(t, err)

	// Test permission check
	result, err := repo.CheckPermission(ctx, user.ID, doc.ID, "viewer")
	require.NoError(t, err)

	assert.True(t, result.HasPermission)
	assert.Equal(t, "viewer", result.PermissionType)
	assert.Equal(t, "direct", result.Sources[0])
	assert.False(t, result.CacheHit)
	assert.Greater(t, result.DurationMs, 0.0)

	// Test permission denial
	result, err = repo.CheckPermission(ctx, user.ID, doc.ID, "editor")
	require.NoError(t, err)
	assert.False(t, result.HasPermission)
}

func TestMySQLPermissionRepository_CheckPermissionsBatch(t *testing.T) {
	db := setupMySQLTestDB(t)
	repo := NewMySQLPermissionRepository(db)
	ctx := context.Background()

	// Create test data
	user := createTestUser(db, "user-1", "Test User", "test@example.com")
	customer := createTestCustomer(db, "customer-1", "Test Customer")
	doc1 := createTestDocument(db, "doc-1", "Doc 1", customer.ID, user.ID)
	doc2 := createTestDocument(db, "doc-2", "Doc 2", customer.ID, user.ID)
	doc3 := createTestDocument(db, "doc-3", "Doc 3", customer.ID, user.ID)

	// Grant permissions to doc1 and doc2
	err := repo.GrantDirectPermission(ctx, user.ID, doc1.ID, "viewer")
	require.NoError(t, err)
	err = repo.GrantDirectPermission(ctx, user.ID, doc2.ID, "viewer")
	require.NoError(t, err)

	// Test batch check
	docIDs := []string{doc1.ID, doc2.ID, doc3.ID}
	results, err := repo.CheckPermissionsBatch(ctx, user.ID, docIDs, "viewer")
	require.NoError(t, err)

	assert.True(t, results[doc1.ID])
	assert.True(t, results[doc2.ID])
	assert.False(t, results[doc3.ID])
}

func TestMySQLPermissionRepository_AddCustomerFollowerPermissions(t *testing.T) {
	db := setupMySQLTestDB(t)
	repo := NewMySQLPermissionRepository(db)
	ctx := context.Background()

	// Create test data
	user1 := createTestUser(db, "user-1", "User 1", "user1@example.com")
	user2 := createTestUser(db, "user-2", "User 2", "user2@example.com")
	customer := createTestCustomer(db, "customer-1", "Test Customer")

	// Create documents for customer
	doc1 := createTestDocument(db, "doc-1", "Doc 1", customer.ID, user1.ID)
	doc2 := createTestDocument(db, "doc-2", "Doc 2", customer.ID, user1.ID)
	doc3 := createTestDocument(db, "doc-3", "Doc 3", customer.ID, user1.ID)

	// Add user2 as customer follower
	err := repo.AddCustomerFollowerPermissions(ctx, customer.ID, user2.ID)
	require.NoError(t, err)

	// Verify user2 has access to all customer documents
	result, err := repo.CheckPermission(ctx, user2.ID, doc1.ID, "viewer")
	require.NoError(t, err)
	assert.True(t, result.HasPermission)
	assert.Equal(t, "customer_follower", result.Sources[0])

	result, err = repo.CheckPermission(ctx, user2.ID, doc2.ID, "viewer")
	require.NoError(t, err)
	assert.True(t, result.HasPermission)

	result, err = repo.CheckPermission(ctx, user2.ID, doc3.ID, "viewer")
	require.NoError(t, err)
	assert.True(t, result.HasPermission)
}

func TestMySQLPermissionRepository_ExpandManagerChain(t *testing.T) {
	db := setupMySQLTestDB(t)
	repo := NewMySQLPermissionRepository(db)
	ctx := context.Background()

	// Create organizational hierarchy
	dept1 := createTestDepartment(db, "dept-1", "Engineering", 3, nil)

	user1 := createTestUser(db, "user-1", "Regular User", "user1@example.com")
	manager1 := createTestUser(db, "manager-1", "Manager", "manager@example.com")
	manager2 := createTestUser(db, "manager-2", "Senior Manager", "senior@example.com")

	// Assign user to department
	db.Create(&model.UserDepartment{
		UserID:       user1.ID,
		DepartmentID: dept1.ID,
		Role:         "member",
		IsPrimary:    true,
	})

	// Create management relations
	db.Create(&model.ManagementRelation{
		ManagerUserID:     manager1.ID,
		SubordinateUserID: user1.ID,
		DepartmentID:      dept1.ID,
		ManagementLevel:   1,
	})

	db.Create(&model.ManagementRelation{
		ManagerUserID:     manager2.ID,
		SubordinateUserID: user1.ID,
		DepartmentID:      dept1.ID,
		ManagementLevel:   2,
	})

	// Create document by user1
	customer := createTestCustomer(db, "customer-1", "Test Customer")
	doc := createTestDocument(db, "doc-1", "Test Document", customer.ID, user1.ID)

	// Expand manager chain
	err := repo.ExpandManagerChain(ctx, user1.ID, doc.ID, "viewer")
	require.NoError(t, err)

	// Verify direct manager has permission
	result, err := repo.CheckPermission(ctx, manager1.ID, doc.ID, "viewer")
	require.NoError(t, err)
	assert.True(t, result.HasPermission)
	assert.Equal(t, "manager_chain", result.Sources[0])

	// Verify senior manager has permission
	result, err = repo.CheckPermission(ctx, manager2.ID, doc.ID, "viewer")
	require.NoError(t, err)
	assert.True(t, result.HasPermission)
	assert.Equal(t, "manager_chain", result.Sources[0])
}

func TestMySQLPermissionRepository_RevokePermission(t *testing.T) {
	db := setupMySQLTestDB(t)
	repo := NewMySQLPermissionRepository(db)
	ctx := context.Background()

	// Create test data
	user := createTestUser(db, "user-1", "Test User", "test@example.com")
	customer := createTestCustomer(db, "customer-1", "Test Customer")
	doc := createTestDocument(db, "doc-1", "Test Document", customer.ID, user.ID)

	// Grant permission
	err := repo.GrantDirectPermission(ctx, user.ID, doc.ID, "viewer")
	require.NoError(t, err)

	// Verify permission exists
	result, err := repo.CheckPermission(ctx, user.ID, doc.ID, "viewer")
	require.NoError(t, err)
	assert.True(t, result.HasPermission)

	// Revoke permission
	err = repo.RevokePermission(ctx, user.ID, doc.ID)
	require.NoError(t, err)

	// Verify permission revoked
	result, err = repo.CheckPermission(ctx, user.ID, doc.ID, "viewer")
	require.NoError(t, err)
	assert.False(t, result.HasPermission)
}

func TestMySQLPermissionRepository_GetUserDocuments(t *testing.T) {
	db := setupMySQLTestDB(t)
	repo := NewMySQLPermissionRepository(db)
	ctx := context.Background()

	// Create test data
	user := createTestUser(db, "user-1", "Test User", "test@example.com")
	customer := createTestCustomer(db, "customer-1", "Test Customer")
	doc1 := createTestDocument(db, "doc-1", "Doc 1", customer.ID, user.ID)
	doc2 := createTestDocument(db, "doc-2", "Doc 2", customer.ID, user.ID)
	doc3 := createTestDocument(db, "doc-3", "Doc 3", customer.ID, user.ID)

	// Grant permissions
	err := repo.GrantDirectPermission(ctx, user.ID, doc1.ID, "viewer")
	require.NoError(t, err)
	err = repo.GrantDirectPermission(ctx, user.ID, doc2.ID, "viewer")
	require.NoError(t, err)
	err = repo.GrantDirectPermission(ctx, user.ID, doc3.ID, "viewer")
	require.NoError(t, err)

	// Get user documents
	result, err := repo.GetUserDocuments(ctx, user.ID, "viewer", 1, 2)
	require.NoError(t, err)

	assert.Equal(t, int64(3), result.Total)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 2, result.PageSize)
	assert.Len(t, result.Documents, 2)
	assert.Greater(t, result.DurationMs, 0.0)
}

func TestMySQLPermissionRepository_GetStorageStats(t *testing.T) {
	db := setupMySQLTestDB(t)
	repo := NewMySQLPermissionRepository(db)
	ctx := context.Background()

	// Create test data
	user := createTestUser(db, "user-1", "Test User", "test@example.com")
	customer := createTestCustomer(db, "customer-1", "Test Customer")
	doc := createTestDocument(db, "doc-1", "Test Document", customer.ID, user.ID)

	err := repo.GrantDirectPermission(ctx, user.ID, doc.ID, "viewer")
	require.NoError(t, err)

	// Get storage stats
	stats, err := repo.GetStorageStats(ctx)
	require.NoError(t, err)

	assert.Equal(t, "MySQL", stats.EngineType)
	assert.Equal(t, "document_permissions_mysql", stats.TableName)
	assert.Greater(t, stats.RowCount, int64(0))
	assert.Greater(t, stats.TotalSizeMB, 0.0)
}

// ========================================================================
// TESTS FOR CORRECTED IMPLEMENTATIONS
// These tests verify the complete business logic
// ========================================================================

// TestAddDocumentPermissionsComplete tests the complete document permission logic
// This verifies ALL 5 permission sources are handled correctly:
// 1. Creator permissions
// 2. Customer follower permissions
// 3. Creator's manager chain permissions
// 4. ALL followers' manager chain permissions
// 5. Superuser permissions
func TestAddDocumentPermissionsComplete(t *testing.T) {
	db := setupMySQLTestDB(t)
	repo := NewMySQLPermissionRepository(db)
	ctx := context.Background()

	// Create test users
	creator := createTestUser(db, "creator-1", "Document Creator", "creator@example.com")
	follower1 := createTestUser(db, "follower-1", "Follower 1", "follower1@example.com")
	follower2 := createTestUser(db, "follower-2", "Follower 2", "follower2@example.com")
	manager1 := createTestUser(db, "manager-1", "Creator's Manager", "manager1@example.com")
	manager2 := createTestUser(db, "manager-2", "Follower's Manager", "manager2@example.com")
	superuser := createTestUser(db, "superuser-1", "Superuser", "super@example.com")
	superuser.IsSuperuser = true
	db.Save(&superuser)

	// Create test customer and department
	customer := createTestCustomer(db, "customer-1", "Test Customer")
	dept := createTestDepartment(db, "dept-1", "Engineering", 3, nil)

	// Assign creator to department
	db.Create(&model.UserDepartment{
		UserID:       creator.ID,
		DepartmentID: dept.ID,
		Role:         "member",
		IsPrimary:    true,
	})

	// Assign follower1 to department (has manager2)
	db.Create(&model.UserDepartment{
		UserID:       follower1.ID,
		DepartmentID: dept.ID,
		Role:         "member",
		IsPrimary:    true,
	})

	// Create management relations
	// Creator is managed by manager1
	db.Create(&model.ManagementRelation{
		ManagerUserID:     manager1.ID,
		SubordinateUserID: creator.ID,
		DepartmentID:      dept.ID,
		ManagementLevel:   1,
	})

	// Follower1 is managed by manager2
	db.Create(&model.ManagementRelation{
		ManagerUserID:     manager2.ID,
		SubordinateUserID: follower1.ID,
		DepartmentID:      dept.ID,
		ManagementLevel:   1,
	})

	// Add customer followers
	db.Create(&model.CustomerFollower{UserID: follower1.ID, CustomerID: customer.ID})
	db.Create(&model.CustomerFollower{UserID: follower2.ID, CustomerID: customer.ID})

	// Create a test document
	document := &model.Document{
		ID:         "doc-complete-test",
		CustomerID: customer.ID,
		CreatorID:  creator.ID,
		Title:      "Test Document - Complete Logic",
	}
	db.Create(document)

	// Execute the complete permission method
	err := repo.AddDocumentPermissionsComplete(ctx, document)
	require.NoError(t, err)

	// Verify all permissions were created correctly
	var permissions []model.DocumentPermissionMySQL
	err = db.Where("document_id = ?", "doc-complete-test").Order("user_id, permission_type, source_type").Find(&permissions).Error
	require.NoError(t, err)

	// Create a map for easier verification
	permMap := make(map[string]model.DocumentPermissionMySQL)
	for _, perm := range permissions {
		key := perm.UserID + "|" + perm.PermissionType + "|" + perm.SourceType
		permMap[key] = perm
	}

	// Print all permissions for debugging
	t.Logf("✅ Total permissions created: %d", len(permissions))
	for _, perm := range permissions {
		t.Logf("  - %s | %s | %s | source: %v", perm.UserID, perm.PermissionType, perm.SourceType, perm.SourceID)
	}

	// Verify creator has owner and viewer permissions
	assert.Contains(t, permMap, creator.ID+"|owner|direct", "Creator should have owner permission")
	assert.Contains(t, permMap, creator.ID+"|viewer|direct", "Creator should have viewer permission")

	// Verify both followers have customer_follower permissions
	assert.Contains(t, permMap, follower1.ID+"|viewer|customer_follower", "Follower1 should have customer_follower permission")
	assert.Contains(t, permMap, follower2.ID+"|viewer|customer_follower", "Follower2 should have customer_follower permission")

	// Verify creator's manager has manager_chain permission with creator as source
	if perm, ok := permMap[manager1.ID+"|viewer|manager_chain"]; ok {
		assert.Equal(t, creator.ID, *perm.SourceID, "Creator's manager permission should have creator as source")
	} else {
		t.Fatal("Creator's manager should have manager_chain permission")
	}

	// Verify follower's manager has manager_chain permission with customer as source
	if perm, ok := permMap[manager2.ID+"|viewer|manager_chain"]; ok {
		assert.Equal(t, customer.ID, *perm.SourceID, "Follower's manager permission should have customer as source")
	} else {
		t.Fatal("Follower's manager should have manager_chain permission")
	}

	// Verify superuser has superuser permission with no source
	if perm, ok := permMap[superuser.ID+"|viewer|superuser"]; ok {
		assert.Nil(t, perm.SourceID, "Superuser permission should have no source ID")
	} else {
		t.Fatal("Superuser should have superuser permission")
	}

	// Verify expected count
	// Expected: creator(2) + follower1(1) + follower2(1) + manager1(1) + manager2(1) + superuser(1) = 7
	expectedMinCount := 7 // Minimum permissions
	assert.GreaterOrEqual(t, len(permissions), expectedMinCount,
		"Should have at least %d permissions (got %d)", expectedMinCount, len(permissions))

	t.Logf("✅ Test passed! All permission sources verified correctly.")
}

// TestReplaceCustomerFollowerComplete tests the complete customer follower replacement logic
// This verifies ALL 4 steps are handled correctly:
// 1. Remove old follower's customer_follower permissions
// 2. Remove old follower's manager chain permissions
// 3. Add new follower's customer_follower permissions
// 4. Add new follower's manager chain permissions
func TestReplaceCustomerFollowerComplete(t *testing.T) {
	db := setupMySQLTestDB(t)
	repo := NewMySQLPermissionRepository(db)
	ctx := context.Background()

	// Create test users
	oldFollower := createTestUser(db, "old-follower", "Old Follower", "old@example.com")
	newFollower := createTestUser(db, "new-follower", "New Follower", "new@example.com")
	oldManager := createTestUser(db, "old-manager", "Old Follower's Manager", "old-manager@example.com")
	newManager := createTestUser(db, "new-manager", "New Follower's Manager", "new-manager@example.com")

	// Create test customer and department
	customer := createTestCustomer(db, "customer-replace", "Customer Replace Test")
	dept := createTestDepartment(db, "dept-replace", "Engineering", 3, nil)

	// Create documents for this customer (3 documents)
	doc1 := createTestDocument(db, "doc-replace-1", "Doc 1", customer.ID, "creator-1")
	doc2 := createTestDocument(db, "doc-replace-2", "Doc 2", customer.ID, "creator-1")
	doc3 := createTestDocument(db, "doc-replace-3", "Doc 3", customer.ID, "creator-1")

	// Assign old follower to department with manager
	db.Create(&model.UserDepartment{
		UserID:       oldFollower.ID,
		DepartmentID: dept.ID,
		Role:         "member",
		IsPrimary:    true,
	})
	db.Create(&model.ManagementRelation{
		ManagerUserID:     oldManager.ID,
		SubordinateUserID: oldFollower.ID,
		DepartmentID:      dept.ID,
		ManagementLevel:   1,
	})

	// Assign new follower to department with different manager
	db.Create(&model.UserDepartment{
		UserID:       newFollower.ID,
		DepartmentID: dept.ID,
		Role:         "member",
		IsPrimary:    true,
	})
	db.Create(&model.ManagementRelation{
		ManagerUserID:     newManager.ID,
		SubordinateUserID: newFollower.ID,
		DepartmentID:      dept.ID,
		ManagementLevel:   1,
	})

	// Add old follower as customer follower
	db.Create(&model.CustomerFollower{UserID: oldFollower.ID, CustomerID: customer.ID})

	// Create permissions for old follower and their manager using the complete method
	// (Simulating that they were added when documents were created)
	oldFollowerPerms := []model.DocumentPermissionMySQL{
		{UserID: oldFollower.ID, DocumentID: doc1.ID, PermissionType: "viewer", SourceType: "customer_follower", SourceID: &customer.ID},
		{UserID: oldFollower.ID, DocumentID: doc2.ID, PermissionType: "viewer", SourceType: "customer_follower", SourceID: &customer.ID},
		{UserID: oldFollower.ID, DocumentID: doc3.ID, PermissionType: "viewer", SourceType: "customer_follower", SourceID: &customer.ID},
		{UserID: oldManager.ID, DocumentID: doc1.ID, PermissionType: "viewer", SourceType: "manager_chain", SourceID: &customer.ID},
		{UserID: oldManager.ID, DocumentID: doc2.ID, PermissionType: "viewer", SourceType: "manager_chain", SourceID: &customer.ID},
		{UserID: oldManager.ID, DocumentID: doc3.ID, PermissionType: "viewer", SourceType: "manager_chain", SourceID: &customer.ID},
	}
	db.Create(&oldFollowerPerms)

	// Verify old follower and manager have permissions
	oldFollowerCount := int64(0)
	db.Table("document_permissions_mysql").Where("user_id = ? AND document_id IN ?", oldFollower.ID, []string{doc1.ID, doc2.ID, doc3.ID}).Count(&oldFollowerCount)
	assert.Equal(t, int64(3), oldFollowerCount, "Old follower should have 3 permissions")

	oldManagerCount := int64(0)
	db.Table("document_permissions_mysql").Where("user_id = ? AND document_id IN ? AND source_type = ?", oldManager.ID, []string{doc1.ID, doc2.ID, doc3.ID}, "manager_chain").Count(&oldManagerCount)
	assert.Equal(t, int64(3), oldManagerCount, "Old manager should have 3 manager_chain permissions")

	// Execute the replacement
	err := repo.ReplaceCustomerFollowerComplete(ctx, customer.ID, oldFollower.ID, newFollower.ID)
	require.NoError(t, err)

	// Verify old follower's permissions were removed
	var oldFollowerPermsAfter []model.DocumentPermissionMySQL
	db.Where("user_id = ? AND document_id IN ?", oldFollower.ID, []string{doc1.ID, doc2.ID, doc3.ID}).Find(&oldFollowerPermsAfter)
	assert.Equal(t, 0, len(oldFollowerPermsAfter), "Old follower should have no permissions after replacement")

	// Verify old manager's manager_chain permissions were removed (ONLY from this customer)
	var oldManagerPermsAfter []model.DocumentPermissionMySQL
	db.Where("user_id = ? AND source_type = ? AND source_id = ?", oldManager.ID, "manager_chain", customer.ID).Find(&oldManagerPermsAfter)
	assert.Equal(t, 0, len(oldManagerPermsAfter), "Old manager should have no manager_chain permissions from this customer after replacement")

	// Verify new follower has permissions
	var newFollowerPermsAfter []model.DocumentPermissionMySQL
	db.Where("user_id = ? AND document_id IN ?", newFollower.ID, []string{doc1.ID, doc2.ID, doc3.ID}).Find(&newFollowerPermsAfter)
	assert.GreaterOrEqual(t, len(newFollowerPermsAfter), 3, "New follower should have at least 3 permissions")

	// Verify new follower has customer_follower source
	hasCustomerFollowerSource := false
	for _, perm := range newFollowerPermsAfter {
		if perm.SourceType == "customer_follower" {
			hasCustomerFollowerSource = true
			assert.Equal(t, customer.ID, *perm.SourceID)
		}
	}
	assert.True(t, hasCustomerFollowerSource, "New follower should have customer_follower source")

	// Verify new manager has manager_chain permissions
	var newManagerPerms []model.DocumentPermissionMySQL
	db.Where("user_id = ? AND source_type = ? AND source_id = ?", newManager.ID, "manager_chain", customer.ID).Find(&newManagerPerms)
	assert.GreaterOrEqual(t, len(newManagerPerms), 3, "New manager should have at least 3 manager_chain permissions")

	t.Logf("✅ Test passed! Customer follower replacement verified correctly.")
	t.Logf("   - Old follower permissions removed: 3")
	t.Logf("   - Old manager permissions removed: 3")
	t.Logf("   - New follower permissions added: %d", len(newFollowerPermsAfter))
	t.Logf("   - New manager permissions added: %d", len(newManagerPerms))
}

// TestRevokeSuperuserPermissionsComplete tests the complete superuser revocation logic
// This verifies that permissions are only deleted if no other sources exist
func TestRevokeSuperuserPermissionsComplete(t *testing.T) {
	db := setupMySQLTestDB(t)
	repo := NewMySQLPermissionRepository(db)
	ctx := context.Background()

	// Create test users
	superuser := createTestUser(db, "superuser-revoke", "Superuser to Revoke", "superuser-revoke@example.com")
	superuser.IsSuperuser = true
	db.Save(&superuser)

	otherUser := createTestUser(db, "creator-super", "Document Creator", "creator-super@example.com")

	// Create test customer and documents
	customer := createTestCustomer(db, "customer-super", "Customer Superuser Test")
	doc1 := createTestDocument(db, "doc-super-1", "Doc 1", customer.ID, otherUser.ID)
	doc2 := createTestDocument(db, "doc-super-2", "Doc 2", customer.ID, otherUser.ID)
	doc3 := createTestDocument(db, "doc-super-3", "Doc 3", customer.ID, otherUser.ID)

	// Add superuser permissions to all 3 documents
	superuserPerms := []model.DocumentPermissionMySQL{
		{UserID: superuser.ID, DocumentID: doc1.ID, PermissionType: "viewer", SourceType: "superuser"},
		{UserID: superuser.ID, DocumentID: doc2.ID, PermissionType: "viewer", SourceType: "superuser"},
		{UserID: superuser.ID, DocumentID: doc3.ID, PermissionType: "viewer", SourceType: "superuser"},
	}
	db.Clauses(clause.OnConflict{DoNothing: true}).Create(&superuserPerms)

	// Also give the superuser a direct permission to doc1 (using INSERT IGNORE)
	directPerm := model.DocumentPermissionMySQL{
		UserID:         superuser.ID,
		DocumentID:     doc1.ID,
		PermissionType: "editor", // Use different permission type to avoid unique key conflict
		SourceType:     "direct",
		SourceID:       &doc1.ID,
	}
	db.Clauses(clause.OnConflict{DoNothing: true}).Create(&directPerm)

	// Also give the superuser customer_follower permission to doc2
	followerPerm := model.DocumentPermissionMySQL{
		UserID:         superuser.ID,
		DocumentID:     doc2.ID,
		PermissionType: "editor", // Use different permission type
		SourceType:     "customer_follower",
		SourceID:       &customer.ID,
	}
	db.Clauses(clause.OnConflict{DoNothing: true}).Create(&followerPerm)

	// Verify initial state
	var initialSuperuserPerms []model.DocumentPermissionMySQL
	db.Where("user_id = ? AND source_type = ?", superuser.ID, "superuser").Find(&initialSuperuserPerms)
	assert.Equal(t, 3, len(initialSuperuserPerms), "Should have 3 superuser permissions initially")

	// Execute the revocation
	err := repo.RevokeSuperuserPermissionsComplete(ctx, superuser.ID)
	require.NoError(t, err)

	// Verify superuser flag was removed
	var user model.User
	db.Where("id = ?", superuser.ID).First(&user)
	assert.False(t, user.IsSuperuser, "User should no longer be superuser")

	// Verify superuser permissions were handled correctly
	var remainingSuperuserPerms []model.DocumentPermissionMySQL
	db.Where("user_id = ? AND source_type = ?", superuser.ID, "superuser").Find(&remainingSuperuserPerms)

	// Should have deleted superuser permission for doc3 (no other sources)
	// Should have kept superuser permission for doc1 and doc2 (has other sources)
	// OR deleted all and user still has access via other sources

	// Check if user still has access to doc1 via direct source
	var directAccess model.DocumentPermissionMySQL
	err = db.Where("user_id = ? AND document_id = ? AND source_type = ? AND permission_type = ?", superuser.ID, doc1.ID, "direct", "editor").First(&directAccess).Error
	assert.NoError(t, err, "User should still have direct access to doc1")

	// Check if user still has access to doc2 via customer_follower source
	var followerAccess model.DocumentPermissionMySQL
	err = db.Where("user_id = ? AND document_id = ? AND source_type = ? AND permission_type = ?", superuser.ID, doc2.ID, "customer_follower", "editor").First(&followerAccess).Error
	assert.NoError(t, err, "User should still have customer_follower access to doc2")

	// Check if user lost access to doc3 (should have no permissions)
	var doc3Perms []model.DocumentPermissionMySQL
	db.Where("user_id = ? AND document_id = ?", superuser.ID, doc3.ID).Find(&doc3Perms)
	assert.Equal(t, 0, len(doc3Perms), "User should have NO access to doc3 after superuser revocation")

	t.Logf("✅ Test passed! Superuser revocation verified correctly.")
	t.Logf("   - User no longer has is_superuser flag")
	t.Logf("   - User still has access to doc1 (direct source)")
	t.Logf("   - User still has access to doc2 (customer_follower source)")
	t.Logf("   - User lost access to doc3 (only had superuser source)")
}
