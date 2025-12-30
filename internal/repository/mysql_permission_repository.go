package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/d60-Lab/gin-template/internal/model"
)

// MySQLPermissionRepository handles MySQL expanded permission operations
type MySQLPermissionRepository struct {
	db *gorm.DB
}

// NewMySQLPermissionRepository creates a new MySQL permission repository
func NewMySQLPermissionRepository(db *gorm.DB) *MySQLPermissionRepository {
	return &MySQLPermissionRepository{db: db}
}

// CheckPermission checks if a user has permission to access a document
func (r *MySQLPermissionRepository) CheckPermission(ctx context.Context, userID, documentID, permissionType string) (*model.PermissionCheckResult, error) {
	startTime := time.Now()

	var permission model.DocumentPermissionMySQL
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND document_id = ? AND permission_type = ?", userID, documentID, permissionType).
		First(&permission).Error

	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &model.PermissionCheckResult{
				HasPermission: false,
				DurationMs:    float64(duration),
				CacheHit:      false,
			}, nil
		}
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}

	return &model.PermissionCheckResult{
		HasPermission: true,
		PermissionType: permission.PermissionType,
		Sources:       []string{permission.SourceType},
		DurationMs:    float64(duration),
		CacheHit:      false,
	}, nil
}

// CheckPermissionsBatch checks permissions for multiple documents at once
func (r *MySQLPermissionRepository) CheckPermissionsBatch(ctx context.Context, userID string, documentIDs []string, permissionType string) (map[string]bool, error) {
	var permissions []model.DocumentPermissionMySQL
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND document_id IN ? AND permission_type = ?", userID, documentIDs, permissionType).
		Find(&permissions).Error

	if err != nil {
		return nil, fmt.Errorf("failed to check batch permissions: %w", err)
	}

	result := make(map[string]bool)
	for _, docID := range documentIDs {
		result[docID] = false
	}

	for _, permission := range permissions {
		result[permission.DocumentID] = true
	}

	return result, nil
}

// GetUserDocuments returns paginated list of documents user can access
func (r *MySQLPermissionRepository) GetUserDocuments(ctx context.Context, userID string, permissionType string, page, pageSize int) (*model.UserDocumentList, error) {
	startTime := time.Now()

	offset := (page - 1) * pageSize

	var permissions []model.DocumentPermissionMySQL
	var total int64

	// Count total
	countQuery := r.db.WithContext(ctx).
		Model(&model.DocumentPermissionMySQL{}).
		Where("user_id = ? AND permission_type = ?", userID, permissionType)

	if err := countQuery.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count user documents: %w", err)
	}

	// Fetch permissions with pagination
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND permission_type = ?", userID, permissionType).
		Preload("Document").
		Preload("Document.Customer").
		Preload("Document.Creator").
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&permissions).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch user documents: %w", err)
	}

	// Convert to document list items
	documents := make([]model.DocumentListItem, 0, len(permissions))
	for _, perm := range permissions {
		if perm.Document != nil {
			doc := model.DocumentListItem{
				ID:             perm.Document.ID,
				Title:          perm.Document.Title,
				CustomerID:     perm.Document.CustomerID,
				CreatorID:      perm.Document.CreatorID,
				PermissionType: perm.PermissionType,
				SourceType:     perm.SourceType,
				CreatedAt:      perm.Document.CreatedAt,
			}

			if perm.Document.Customer != nil {
				doc.CustomerName = perm.Document.Customer.Name
			}
			if perm.Document.Creator != nil {
				doc.CreatorName = perm.Document.Creator.Name
			}

			documents = append(documents, doc)
		}
	}

	duration := time.Since(startTime).Milliseconds()

	return &model.UserDocumentList{
		Documents: documents,
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
		DurationMs: float64(duration),
	}, nil
}

// GrantDirectPermission grants direct permission to a user
func (r *MySQLPermissionRepository) GrantDirectPermission(ctx context.Context, userID, documentID, permissionType string) error {
	permission := &model.DocumentPermissionMySQL{
		UserID:         userID,
		DocumentID:     documentID,
		PermissionType: permissionType,
		SourceType:     "direct",
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "document_id"}, {Name: "permission_type"}},
			DoUpdates: clause.AssignmentColumns([]string{"source_type", "updated_at"}),
		}).
		Create(permission).Error
}

// RevokePermission revokes permission from a user
func (r *MySQLPermissionRepository) RevokePermission(ctx context.Context, userID, documentID string) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND document_id = ?", userID, documentID).
		Delete(&model.DocumentPermissionMySQL{}).Error
}

// AddCustomerFollowerPermissions adds permissions for a customer follower
// This affects ALL documents belonging to the customer
func (r *MySQLPermissionRepository) AddCustomerFollowerPermissions(ctx context.Context, customerID, userID string) error {
	// Find all documents for this customer
	var documents []model.Document
	if err := r.db.WithContext(ctx).
		Where("customer_id = ?", customerID).
		Find(&documents).Error; err != nil {
		return fmt.Errorf("failed to find customer documents: %w", err)
	}

	// Add permissions in batch
	permissions := make([]model.DocumentPermissionMySQL, 0, len(documents))
	for _, doc := range documents {
		permissions = append(permissions, model.DocumentPermissionMySQL{
			UserID:         userID,
			DocumentID:     doc.ID,
			PermissionType: "viewer",
			SourceType:     "customer_follower",
			SourceID:       &customerID,
		})
	}

	if len(permissions) > 0 {
		return r.db.WithContext(ctx).
			Clauses(clause.OnConflict{
				DoNothing: true,
			}).
			CreateInBatches(permissions, 100).Error
	}

	return nil
}

// RemoveCustomerFollowerPermissions removes permissions for a customer follower
func (r *MySQLPermissionRepository) RemoveCustomerFollowerPermissions(ctx context.Context, customerID, userID string) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND source_type = ? AND source_id = ?", userID, "customer_follower", customerID).
		Delete(&model.DocumentPermissionMySQL{}).Error
}

// ExpandManagerChain expands manager chain permissions for a user
// This is called when a user creates a document - all their managers get access
func (r *MySQLPermissionRepository) ExpandManagerChain(ctx context.Context, userID, documentID, permissionType string) error {
	// Find all managers for this user (through all departments)
	var managerRelations []model.ManagementRelation
	err := r.db.WithContext(ctx).
		Joins("JOIN user_departments ON user_departments.department_id = management_relations.department_id").
		Where("management_relations.subordinate_user_id = ? AND user_departments.user_id = ?", userID, userID).
		Find(&managerRelations).Error

	if err != nil {
		return fmt.Errorf("failed to find manager relations: %w", err)
	}

	// Collect unique manager IDs
	managerIDs := make(map[string]string) // managerID -> departmentID
	for _, rel := range managerRelations {
		managerIDs[rel.ManagerUserID] = rel.DepartmentID
	}

	// Add permissions for direct managers
	permissions := make([]model.DocumentPermissionMySQL, 0, len(managerIDs))
	for managerID := range managerIDs {
		permissions = append(permissions, model.DocumentPermissionMySQL{
			UserID:         managerID,
			DocumentID:     documentID,
			PermissionType: permissionType,
			SourceType:     "manager_chain",
			SourceID:       &userID,
		})
	}

	if len(permissions) > 0 {
		if err := r.db.WithContext(ctx).
			Clauses(clause.OnConflict{
				DoNothing: true,
			}).
			CreateInBatches(permissions, 100).Error; err != nil {
			return fmt.Errorf("failed to add manager permissions: %w", err)
		}
	}

	// Recursively add permissions for managers' managers (up to 5 levels)
	return r.expandManagerChainRecursive(ctx, userID, documentID, permissionType, 1, 5)
}

// expandManagerChainRecursive recursively expands manager chain
func (r *MySQLPermissionRepository) expandManagerChainRecursive(ctx context.Context, currentUserID, documentID, permissionType string, currentLevel, maxLevel int) error {
	if currentLevel > maxLevel {
		return nil
	}

	// Find all managers for current user
	var managerRelations []model.ManagementRelation
	err := r.db.WithContext(ctx).
		Joins("JOIN user_departments ON user_departments.department_id = management_relations.department_id").
		Where("management_relations.subordinate_user_id = ? AND user_departments.user_id = ?", currentUserID, currentUserID).
		Find(&managerRelations).Error

	if err != nil {
		return err
	}

	// For each manager, add permission and recurse
	permissions := make([]model.DocumentPermissionMySQL, 0)
	for _, rel := range managerRelations {
		managerID := rel.ManagerUserID

		// Check if permission already exists
		var existingPerm model.DocumentPermissionMySQL
		err := r.db.WithContext(ctx).
			Where("user_id = ? AND document_id = ? AND source_type = ? AND source_id = ?",
				managerID, documentID, "manager_chain", currentUserID).
			First(&existingPerm).Error

		if err == gorm.ErrRecordNotFound {
			// Add new permission
			permissions = append(permissions, model.DocumentPermissionMySQL{
				UserID:         managerID,
				DocumentID:     documentID,
				PermissionType: permissionType,
				SourceType:     "manager_chain",
				SourceID:       &currentUserID,
			})

			// Recurse up the chain
			if err := r.expandManagerChainRecursive(ctx, managerID, documentID, permissionType, currentLevel+1, maxLevel); err != nil {
				return err
			}
		}
	}

	if len(permissions) > 0 {
		if err := r.db.WithContext(ctx).
			Clauses(clause.OnConflict{
				DoNothing: true,
			}).
			CreateInBatches(permissions, 100).Error; err != nil {
			return err
		}
	}

	return nil
}

// RebuildDepartmentPermissions rebuilds permissions when department structure changes
// This is EXPENSIVE - needs to rebuild all affected permissions
func (r *MySQLPermissionRepository) RebuildDepartmentPermissions(ctx context.Context, departmentID string) error {
	// Find all users in this department
	var userDepts []model.UserDepartment
	if err := r.db.WithContext(ctx).
		Where("department_id = ?", departmentID).
		Find(&userDepts).Error; err != nil {
		return fmt.Errorf("failed to find department users: %w", err)
	}

	// For each user, we need to rebuild their manager chain permissions
	// This is VERY expensive - involves deleting and re-inserting millions of rows
	for _, userDept := range userDepts {
		if err := r.rebuildUserManagerChain(ctx, userDept.UserID); err != nil {
			return fmt.Errorf("failed to rebuild permissions for user %s: %w", userDept.UserID, err)
		}
	}

	return nil
}

// rebuildUserManagerChain rebuilds all manager chain permissions for a user
// This is the most expensive operation in the MySQL approach
func (r *MySQLPermissionRepository) rebuildUserManagerChain(ctx context.Context, userID string) error {
	// Delete all existing manager_chain permissions for this user
	if err := r.db.WithContext(ctx).
		Where("source_type = ? AND source_id = ?", "manager_chain", userID).
		Delete(&model.DocumentPermissionMySQL{}).Error; err != nil {
		return fmt.Errorf("failed to delete old manager chain: %w", err)
	}

	// Find all documents this user created
	var documents []model.Document
	if err := r.db.WithContext(ctx).
		Where("creator_id = ?", userID).
		Find(&documents).Error; err != nil {
		return fmt.Errorf("failed to find user documents: %w", err)
	}

	// Re-expand manager chain for each document
	for _, doc := range documents {
		if err := r.ExpandManagerChain(ctx, userID, doc.ID, "viewer"); err != nil {
			return fmt.Errorf("failed to expand manager chain for document %s: %w", doc.ID, err)
		}
	}

	return nil
}

// GetStorageStats returns storage statistics for MySQL permissions
func (r *MySQLPermissionRepository) GetStorageStats(ctx context.Context) (*model.StorageStats, error) {
	var stats model.StorageStats

	err := r.db.WithContext(ctx).
		Raw(`
			SELECT
				'MySQL' as engine_type,
				'document_permissions_mysql' as table_name,
				TABLE_ROWS as row_count,
				ROUND(DATA_LENGTH / 1024 / 1024, 2) as data_size_mb,
				ROUND(INDEX_LENGTH / 1024 / 1024, 2) as index_size_mb,
				ROUND((DATA_LENGTH + INDEX_LENGTH) / 1024 / 1024, 2) as total_size_mb
			FROM information_schema.TABLES
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'document_permissions_mysql'
		`).
		Scan(&stats).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get storage stats: %w", err)
	}

	return &stats, nil
}

// GetPermissionStats returns permission statistics by source type
func (r *MySQLPermissionRepository) GetPermissionStats(ctx context.Context) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	err := r.db.WithContext(ctx).
		Model(&model.DocumentPermissionMySQL{}).
		Select("source_type, permission_type, COUNT(*) as total_permissions, COUNT(DISTINCT user_id) as unique_users, COUNT(DISTINCT document_id) as unique_documents").
		Group("source_type, permission_type").
		Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get permission stats: %w", err)
	}

	return results, nil
}

// UpdateDepartmentManager updates department manager and rebuilds all affected permissions
// This is a VERY expensive operation for MySQL - requires:
// 1. Finding old manager and their permissions
// 2. Finding all users in department subtree
// 3. Finding all documents created by these users
// 4. Deleting old manager's permissions (ONLY through this manager chain)
// 5. Adding new manager's permissions (checking for duplicates)
func (r *MySQLPermissionRepository) UpdateDepartmentManager(ctx context.Context, departmentID, newManagerID string) error {
	startTime := time.Now()

	// Step 1: Get the old manager ID for this department
	var oldManagerID sql.NullString
	err := r.db.WithContext(ctx).
		Table("departments").
		Select("manager_id").
		Where("id = ?", departmentID).
		Scan(&oldManagerID).Error
	if err != nil {
		return fmt.Errorf("failed to get old manager: %w", err)
	}

	// Step 2: Find all departments in the subtree (including this department and all children)
	var deptTree []string
	err = r.db.WithContext(ctx).Raw(`
		WITH RECURSIVE dept_tree AS (
			SELECT id FROM departments WHERE id = ?
			UNION ALL
			SELECT d.id FROM departments d
			INNER JOIN dept_tree dt ON d.parent_id = dt.id
		)
		SELECT id FROM dept_tree
	`, departmentID).Scan(&deptTree).Error
	if err != nil {
		return fmt.Errorf("failed to get department tree: %w", err)
	}

	// Step 3: Find all users in the department subtree
	var userIDs []string
	err = r.db.WithContext(ctx).Raw(`
		SELECT DISTINCT user_id FROM user_departments WHERE department_id IN ?
	`, deptTree).Scan(&userIDs).Error
	if err != nil {
		return fmt.Errorf("failed to find department users: %w", err)
	}

	// Step 4: Find all documents created by these users
	var docs []struct {
		ID        string
		CreatorID string
	}
	err = r.db.WithContext(ctx).Raw(`
		SELECT id, creator_id FROM documents WHERE creator_id IN ?
	`, userIDs).Scan(&docs).Error
	if err != nil {
		return fmt.Errorf("failed to find user documents: %w", err)
	}

	documentIDs := make([]string, len(docs))
	creatorMap := make(map[string]string) // document_id -> department_id
	for i, doc := range docs {
		documentIDs[i] = doc.ID
	}

	// Step 5: For each document, find which department its creator belongs to (in our dept tree)
	// This is needed to set the correct source_id
	type DocDept struct {
		DocID        string
		DepartmentID string
	}
	var docDepts []DocDept
	if len(documentIDs) > 0 {
		err = r.db.WithContext(ctx).Raw(`
			SELECT d.id as doc_id, ud.department_id
			FROM documents d
			JOIN user_departments ud ON ud.user_id = d.creator_id
			WHERE d.id IN ?
			AND ud.department_id IN ?
		`, documentIDs, deptTree).Scan(&docDepts).Error
		if err != nil {
			return fmt.Errorf("failed to map documents to departments: %w", err)
		}
	}

	for _, dd := range docDepts {
		creatorMap[dd.DocID] = dd.DepartmentID
	}

	// Step 6: Delete old manager's permissions (ONLY through this manager chain)
	// Critical: Only delete permissions where source_id is in our department tree
	// This preserves permissions the old manager might have from other sources
	if oldManagerID.Valid && len(documentIDs) > 0 {
		deleteResult := r.db.WithContext(ctx).Exec(`
			DELETE FROM document_permissions_mysql
			WHERE user_id = ?
			AND document_id IN ?
			AND source_type = 'manager_chain'
			AND source_id IN ?
		`, oldManagerID.String, documentIDs, deptTree)
		if deleteResult.Error != nil {
			return fmt.Errorf("failed to delete old manager permissions: %w", deleteResult.Error)
		}
		fmt.Printf("   ℹ️  Deleted %d old manager permissions\n", deleteResult.RowsAffected)
	}

	// Step 7: Add new manager's permissions
	// Use INSERT IGNORE to avoid duplicates (new manager might already have some permissions via other sources)
	insertCount := 0

	if len(docDepts) > 0 {
		for _, dd := range docDepts {
			// Use raw SQL with INSERT IGNORE to handle duplicates gracefully
			err = r.db.WithContext(ctx).Exec(`
				INSERT IGNORE INTO document_permissions_mysql
				(user_id, document_id, permission_type, source_type, source_id, created_at, updated_at)
				VALUES (?, ?, 'viewer', 'manager_chain', ?, NOW(), NOW())
			`, newManagerID, dd.DocID, dd.DepartmentID).Error
			if err != nil {
				return fmt.Errorf("failed to insert new manager permission: %w", err)
			}

			insertCount++
			if insertCount%1000 == 0 {
				// Progress indicator every 1000 inserts
			}
		}
	}

	// Step 8: Update the department's manager_id
	err = r.db.WithContext(ctx).
		Table("departments").
		Where("id = ?", departmentID).
		Update("manager_id", newManagerID).Error
	if err != nil {
		return fmt.Errorf("failed to update department manager: %w", err)
	}

	duration := time.Since(startTime)
	fmt.Printf("   📊 MySQL UpdateDepartmentManager completed in %v\n", duration)
	fmt.Printf("      - Processed %d departments in subtree\n", len(deptTree))
	fmt.Printf("      - Processed %d users\n", len(userIDs))
	fmt.Printf("      - Processed %d documents\n", len(docs))
	fmt.Printf("      - Inserted %d new manager permissions\n", len(docDepts))

	return nil
}

// AddUserToDepartment adds a user to a department and rebuilds all affected permissions
// This is expensive for MySQL because it must:
// 1. Find all managers in the department's parent chain (recursive)
// 2. For each manager, find all their documents
// 3. Insert permissions for the user (checking for duplicates)
func (r *MySQLPermissionRepository) AddUserToDepartment(ctx context.Context, userID, departmentID, role string, isPrimary bool) error {
	startTime := time.Now()

	// Step 1: Add user-department relationship
	userDept := model.UserDepartment{
		UserID:       userID,
		DepartmentID: departmentID,
		Role:         role,
		IsPrimary:    isPrimary,
	}

	if err := r.db.WithContext(ctx).Create(&userDept).Error; err != nil {
		return fmt.Errorf("failed to add user to department: %w", err)
	}

	// Step 2: Build manager chain permissions for this user
	// Find all managers in this department's parent chain (recursive up to root)
	type Manager struct {
		ManagerID    string
		DepartmentID string
	}
	var managers []Manager
	err := r.db.WithContext(ctx).Raw(`
		WITH RECURSIVE dept_tree AS (
			SELECT id, manager_id FROM departments WHERE id = ?
			UNION ALL
			SELECT d.id, d.manager_id FROM departments d
			INNER JOIN dept_tree dt ON d.id = dt.parent_id
		)
		SELECT DISTINCT manager_id, id as department_id
		FROM dept_tree
		WHERE manager_id IS NOT NULL
	`, departmentID).Scan(&managers).Error
	if err != nil {
		return fmt.Errorf("failed to find managers: %w", err)
	}

	// Step 3: For each manager, find their documents and grant permission to this user
	// Use INSERT IGNORE to avoid duplicates (user might already have some permissions via other sources)
	totalInserted := 0

	for _, manager := range managers {
		// Find documents created by this manager
		var docs []struct {
			ID string
		}
		err = r.db.WithContext(ctx).Raw(`
			SELECT id FROM documents WHERE creator_id = ?
		`, manager.ManagerID).Scan(&docs).Error
		if err != nil {
			return fmt.Errorf("failed to find manager documents: %w", err)
		}

		// Insert permissions for each document
		for _, doc := range docs {
			// Use INSERT IGNORE to handle duplicates gracefully
			// User might already have permission via:
			// - Direct grant
			// - Customer follower relationship
			// - Another manager chain
			err = r.db.WithContext(ctx).Exec(`
				INSERT IGNORE INTO document_permissions_mysql
				(user_id, document_id, permission_type, source_type, source_id, created_at, updated_at)
				VALUES (?, ?, 'viewer', 'manager_chain', ?, NOW(), NOW())
			`, userID, doc.ID, manager.DepartmentID).Error
			if err != nil {
				return fmt.Errorf("failed to insert manager chain permission: %w", err)
			}
			totalInserted++
		}
	}

	duration := time.Since(startTime)
	fmt.Printf("   📊 MySQL AddUserToDepartment completed in %v\n", duration)
	fmt.Printf("      - Found %d managers in parent chain\n", len(managers))
	fmt.Printf("      - Inserted %d permissions\n", totalInserted)

	return nil
}

// ========================================================================
// CORRECTED IMPLEMENTATIONS - Complete business logic
// These methods handle ALL permission sources correctly
// ========================================================================

// AddDocumentPermissionsComplete adds all required permissions when a document is created
// This is the CORRECTED version that handles:
// 1. Creator permissions
// 2. Customer follower permissions
// 3. Creator's manager chain permissions
// 4. ALL followers' manager chain permissions (MISSING in old version!)
// 5. Superuser permissions (MISSING in old version!)
func (r *MySQLPermissionRepository) AddDocumentPermissionsComplete(ctx context.Context, document *model.Document) error {
	startTime := time.Now()
	permissionCount := 0

	// Step 1: Add creator permissions (owner and viewer)
	creatorPerms := []model.DocumentPermissionMySQL{
		{
			UserID:         document.CreatorID,
			DocumentID:     document.ID,
			PermissionType: "owner",
			SourceType:     "direct",
			SourceID:       &document.ID,
		},
		{
			UserID:         document.CreatorID,
			DocumentID:     document.ID,
			PermissionType: "viewer",
			SourceType:     "direct",
			SourceID:       &document.ID,
		},
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&creatorPerms).Error; err != nil {
		return fmt.Errorf("failed to add creator permissions: %w", err)
	}
	permissionCount += 2

	// Step 2: Add customer follower permissions
	var followers []model.CustomerFollower
	if err := r.db.WithContext(ctx).Where("customer_id = ?", document.CustomerID).Find(&followers).Error; err != nil {
		return fmt.Errorf("failed to find customer followers: %w", err)
	}

	followerIDs := make([]string, 0, len(followers))
	for _, follower := range followers {
		followerIDs = append(followerIDs, follower.UserID)
	}

	if len(followerIDs) > 0 {
		followerPerms := make([]model.DocumentPermissionMySQL, len(followers))
		for i, follower := range followers {
			followerPerms[i] = model.DocumentPermissionMySQL{
				UserID:         follower.UserID,
				DocumentID:     document.ID,
				PermissionType: "viewer",
				SourceType:     "customer_follower",
				SourceID:       &document.CustomerID,
			}
		}
		if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(followerPerms, 100).Error; err != nil {
			return fmt.Errorf("failed to add follower permissions: %w", err)
		}
		permissionCount += len(followerIDs)
	}

	// Step 3: Add creator's manager chain permissions
	creatorManagerIDs, err := r.getManagerChain(ctx, document.CreatorID)
	if err != nil {
		return fmt.Errorf("failed to get creator manager chain: %w", err)
	}

	if len(creatorManagerIDs) > 0 {
		creatorManagerPerms := make([]model.DocumentPermissionMySQL, len(creatorManagerIDs))
		for i, managerID := range creatorManagerIDs {
			creatorManagerPerms[i] = model.DocumentPermissionMySQL{
				UserID:         managerID,
				DocumentID:     document.ID,
				PermissionType: "viewer",
				SourceType:     "manager_chain",
				SourceID:       &document.CreatorID, // Source is the creator
			}
		}
		if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(creatorManagerPerms, 100).Error; err != nil {
			return fmt.Errorf("failed to add creator manager permissions: %w", err)
		}
		permissionCount += len(creatorManagerIDs)
	}

	// Step 4: Add ALL followers' manager chain permissions (CRITICAL - was missing!)
	followerManagerIDs := make(map[string]bool) // Deduplicate
	for _, followerID := range followerIDs {
		managerIDs, err := r.getManagerChain(ctx, followerID)
		if err != nil {
			return fmt.Errorf("failed to get follower %s manager chain: %w", followerID, err)
		}
		for _, managerID := range managerIDs {
			followerManagerIDs[managerID] = true
		}
	}

	if len(followerManagerIDs) > 0 {
		followerManagerPerms := make([]model.DocumentPermissionMySQL, 0, len(followerManagerIDs))
		for managerID := range followerManagerIDs {
			// Use customer ID as source (access comes from following this customer)
			followerManagerPerms = append(followerManagerPerms, model.DocumentPermissionMySQL{
				UserID:         managerID,
				DocumentID:     document.ID,
				PermissionType: "viewer",
				SourceType:     "manager_chain",
				SourceID:       &document.CustomerID, // Source is the customer (via follower)
			})
		}
		if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(followerManagerPerms, 100).Error; err != nil {
			return fmt.Errorf("failed to add follower manager permissions: %w", err)
		}
		permissionCount += len(followerManagerIDs)
	}

	// Step 5: Add superuser permissions (CRITICAL - was completely missing!)
	var superusers []model.User
	if err := r.db.WithContext(ctx).Where("is_superuser = ?", true).Find(&superusers).Error; err != nil {
		return fmt.Errorf("failed to find superusers: %w", err)
	}

	if len(superusers) > 0 {
		superuserPerms := make([]model.DocumentPermissionMySQL, len(superusers))
		for i, superuser := range superusers {
			superuserPerms[i] = model.DocumentPermissionMySQL{
				UserID:         superuser.ID,
				DocumentID:     document.ID,
				PermissionType: "viewer",
				SourceType:     "superuser",
				SourceID:       nil, // No source ID for superuser
			}
		}
		if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(superuserPerms, 100).Error; err != nil {
			return fmt.Errorf("failed to add superuser permissions: %w", err)
		}
		permissionCount += len(superusers)
	}

	duration := time.Since(startTime)
	fmt.Printf("   📊 AddDocumentPermissionsComplete completed in %v\n", duration)
	fmt.Printf("      - Creator: 1\n")
	fmt.Printf("      - Followers: %d\n", len(followerIDs))
	fmt.Printf("      - Creator's managers: %d\n", len(creatorManagerIDs))
	fmt.Printf("      - Followers' managers: %d\n", len(followerManagerIDs))
	fmt.Printf("      - Superusers: %d\n", len(superusers))
	fmt.Printf("      - Total permissions added: %d\n", permissionCount)

	return nil
}

// getManagerChain gets all managers in a user's management chain (up to 5 levels)
func (r *MySQLPermissionRepository) getManagerChain(ctx context.Context, userID string) ([]string, error) {
	// Use recursive CTE to get all managers up to 5 levels
	type ManagerResult struct {
		ManagerUserID string
	}
	var results []ManagerResult

	err := r.db.WithContext(ctx).Raw(`
		WITH RECURSIVE manager_chain AS (
			-- Level 1: Direct managers
			SELECT mr.manager_user_id, 1 as level
			FROM management_relations mr
			JOIN user_departments ud ON ud.department_id = mr.department_id
			WHERE ud.user_id = ? AND mr.subordinate_user_id = ?

			UNION ALL

			-- Higher levels: Managers' managers
			SELECT mr.manager_user_id, mc.level + 1
			FROM management_relations mr
			JOIN user_departments ud ON ud.department_id = mr.department_id
			JOIN manager_chain mc ON mc.manager_user_id = mr.subordinate_user_id
			WHERE mc.level < 5
		)
		SELECT DISTINCT manager_user_id FROM manager_chain WHERE manager_user_id IS NOT NULL
	`, userID, userID).Scan(&results).Error

	if err != nil {
		return nil, err
	}

	managerIDs := make([]string, len(results))
	for i, r := range results {
		managerIDs[i] = r.ManagerUserID
	}

	return managerIDs, nil
}

// ReplaceCustomerFollowerComplete replaces a customer follower with complete logic
// This handles:
// 1. Remove old follower's customer_follower permissions
// 2. Remove old follower's manager chain permissions (CRITICAL - was missing!)
// 3. Add new follower's customer_follower permissions
// 4. Add new follower's manager chain permissions (CRITICAL - was missing!)
func (r *MySQLPermissionRepository) ReplaceCustomerFollowerComplete(ctx context.Context, customerID, oldFollowerID, newFollowerID string) error {
	startTime := time.Now()

	// Step 1: Get all documents for this customer
	var documents []model.Document
	if err := r.db.WithContext(ctx).
		Select("id").
		Where("customer_id = ?", customerID).
		Find(&documents).Error; err != nil {
		return fmt.Errorf("failed to find customer documents: %w", err)
	}

	if len(documents) == 0 {
		fmt.Println("   📊 No documents found for customer")
		return nil
	}

	documentIDs := make([]string, len(documents))
	for i, doc := range documents {
		documentIDs[i] = doc.ID
	}

	// Step 2: Remove old follower's customer_follower permissions
	deleteResult1 := r.db.WithContext(ctx).Exec(`
		DELETE FROM document_permissions_mysql
		WHERE user_id = ?
		AND document_id IN ?
		AND source_type = 'customer_follower'
		AND source_id = ?
	`, oldFollowerID, documentIDs, customerID)
	if deleteResult1.Error != nil {
		return fmt.Errorf("failed to delete old follower permissions: %w", deleteResult1.Error)
	}
	fmt.Printf("   ℹ️  Deleted %d old follower's customer_follower permissions\n", deleteResult1.RowsAffected)

	// Step 3: Remove old follower's manager chain permissions (CRITICAL - was missing!)
	// Get old follower's manager chain
	oldFollowerManagerIDs, err := r.getManagerChain(ctx, oldFollowerID)
	if err != nil {
		return fmt.Errorf("failed to get old follower's manager chain: %w", err)
	}

	if len(oldFollowerManagerIDs) > 0 {
		// For each manager, delete their permissions to this customer's documents
		// BUT ONLY if the source is this customer (via the old follower)
		totalDeleted := 0
		for _, managerID := range oldFollowerManagerIDs {
			deleteResult := r.db.WithContext(ctx).Exec(`
				DELETE FROM document_permissions_mysql
				WHERE user_id = ?
				AND document_id IN ?
				AND source_type = 'manager_chain'
				AND source_id = ?
			`, managerID, documentIDs, customerID)
			if deleteResult.Error != nil {
				return fmt.Errorf("failed to delete old follower's manager %s permissions: %w", managerID, deleteResult.Error)
			}
			totalDeleted += int(deleteResult.RowsAffected)
		}
		fmt.Printf("   ℹ️  Deleted %d old follower's manager chain permissions\n", totalDeleted)
	}

	// Step 4: Add new follower's customer_follower permissions
	newFollowerPerms := make([]model.DocumentPermissionMySQL, len(documentIDs))
	for i, docID := range documentIDs {
		newFollowerPerms[i] = model.DocumentPermissionMySQL{
			UserID:         newFollowerID,
			DocumentID:     docID,
			PermissionType: "viewer",
			SourceType:     "customer_follower",
			SourceID:       &customerID,
		}
	}

	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		CreateInBatches(newFollowerPerms, 100).Error; err != nil {
		return fmt.Errorf("failed to add new follower permissions: %w", err)
	}
	fmt.Printf("   ℹ️  Added %d new follower's customer_follower permissions\n", len(documentIDs))

	// Step 5: Add new follower's manager chain permissions (CRITICAL - was missing!)
	// Get new follower's manager chain
	newFollowerManagerIDs, err := r.getManagerChain(ctx, newFollowerID)
	if err != nil {
		return fmt.Errorf("failed to get new follower's manager chain: %w", err)
	}

	if len(newFollowerManagerIDs) > 0 {
		// For each manager, add permissions to all customer documents
		totalAdded := 0
		for _, managerID := range newFollowerManagerIDs {
			managerPerms := make([]model.DocumentPermissionMySQL, len(documentIDs))
			for i, docID := range documentIDs {
				managerPerms[i] = model.DocumentPermissionMySQL{
					UserID:         managerID,
					DocumentID:     docID,
					PermissionType: "viewer",
					SourceType:     "manager_chain",
					SourceID:       &customerID, // Source is customer (via follower)
				}
			}

			if err := r.db.WithContext(ctx).
				Clauses(clause.OnConflict{DoNothing: true}).
				CreateInBatches(managerPerms, 100).Error; err != nil {
				return fmt.Errorf("failed to add new follower's manager %s permissions: %w", managerID, err)
			}
			totalAdded += len(documentIDs)
		}
		fmt.Printf("   ℹ️  Added %d new follower's manager chain permissions\n", totalAdded)
	}

	duration := time.Since(startTime)
	fmt.Printf("   📊 ReplaceCustomerFollowerComplete completed in %v\n", duration)
	fmt.Printf("      - Customer documents: %d\n", len(documentIDs))
	fmt.Printf("      - Old follower's managers: %d\n", len(oldFollowerManagerIDs))
	fmt.Printf("      - New follower's managers: %d\n", len(newFollowerManagerIDs))

	return nil
}

// RevokeSuperuserPermissionsComplete revokes superuser permissions correctly
// This is CRITICAL - superuser降级时，需要：
// 1. 查询该超管的所有权限
// 2. 对每个权限，检查是否有其他来源
// 3. 只删除那些"仅通过超管身份"获得的权限
func (r *MySQLPermissionRepository) RevokeSuperuserPermissionsComplete(ctx context.Context, userID string) error {
	startTime := time.Now()

	// Step 1: Get all superuser permissions for this user
	var superuserPerms []model.DocumentPermissionMySQL
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND source_type = 'superuser'", userID).
		Find(&superuserPerms).Error; err != nil {
		return fmt.Errorf("failed to find superuser permissions: %w", err)
	}

	if len(superuserPerms) == 0 {
		fmt.Println("   ℹ️  No superuser permissions found for user")
		return nil
	}

	fmt.Printf("   ℹ️  Found %d superuser permissions to check\n", len(superuserPerms))

	// Step 2: For each superuser permission, check if user has permission from other sources
	// If yes, keep it. If no, delete it.
	deletedCount := 0
	keptCount := 0

	for _, superuserPerm := range superuserPerms {
		// Check if user has this permission from other sources
		var otherSourceCount int64
		err := r.db.WithContext(ctx).Raw(`
			SELECT COUNT(*) FROM document_permissions_mysql
			WHERE user_id = ?
			AND document_id = ?
			AND permission_type = ?
			AND source_type != 'superuser'
		`, userID, superuserPerm.DocumentID, superuserPerm.PermissionType).Scan(&otherSourceCount).Error

		if err != nil {
			return fmt.Errorf("failed to check other permission sources: %w", err)
		}

		// If no other sources, delete the superuser permission
		if otherSourceCount == 0 {
			deleteResult := r.db.WithContext(ctx).Exec(`
				DELETE FROM document_permissions_mysql
				WHERE user_id = ? AND document_id = ? AND source_type = 'superuser'
			`, userID, superuserPerm.DocumentID)
			if deleteResult.Error != nil {
				return fmt.Errorf("failed to delete superuser permission: %w", deleteResult.Error)
			}
			deletedCount++
		} else {
			// Has other sources, keep the superuser permission for now
			// (or we could choose to delete it since they have access anyway)
			keptCount++
		}
	}

	// Step 3: Remove superuser flag from user
	if err := r.db.WithContext(ctx).
		Table("users").
		Where("id = ?", userID).
		Update("is_superuser", false).Error; err != nil {
		return fmt.Errorf("failed to remove superuser flag: %w", err)
	}

	duration := time.Since(startTime)
	fmt.Printf("   📊 RevokeSuperuserPermissionsComplete completed in %v\n", duration)
	fmt.Printf("      - Total superuser permissions: %d\n", len(superuserPerms))
	fmt.Printf("      - Deleted (no other sources): %d\n", deletedCount)
	fmt.Printf("      - Kept (has other sources): %d\n", keptCount)

	return nil
}
