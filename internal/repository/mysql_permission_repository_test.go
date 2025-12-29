package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/maynardzanzibar/internal/model"
)

func setupMySQLTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Create tables
	err = db.AutoMigrate(
		&model.User{},
		&model.Department{},
		&model.UserDepartment{},
		&model.ManagementRelation{},
		&model.Customer{},
		&model.Document{},
		&model.CustomerFollower{},
		&model.DocumentPermissionMySQL{},
	)
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
