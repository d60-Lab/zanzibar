package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/maynardzanzibar/internal/model"
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
	startTime := time.Now()

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

	_ = time.Since(startTime).Milliseconds()
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
		if err := r.ExpandManagerChain(ctx, userID, doc.ID, "viewer", nil); err != nil {
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
