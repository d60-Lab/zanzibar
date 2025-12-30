package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/d60-Lab/gin-template/internal/model"
)

// ZanzibarPermissionRepository handles Zanzibar-style tuple-based permissions
type ZanzibarPermissionRepository struct {
	db *gorm.DB
}

// NewZanzibarPermissionRepository creates a new Zanzibar permission repository
func NewZanzibarPermissionRepository(db *gorm.DB) *ZanzibarPermissionRepository {
	return &ZanzibarPermissionRepository{
		db: db,
	}
}

// CheckPermission checks if a user has permission to access a document
// OPTIMIZED: Uses "forward expansion" strategy - expand user's accessible documents first,
// then check if target document is in the set. This is much faster than "backward checking"
// which requires traversing all followers of the document's customer.
func (r *ZanzibarPermissionRepository) CheckPermission(ctx context.Context, userID, documentID, permissionType string) (*model.PermissionCheckResult, error) {
	startTime := time.Now()

	sources := make(model.PermissionSourceList, 0)

	// Path 1: Superuser check (fastest path - single query)
	hasSuperuser, err := r.checkSuperuserPermission(ctx, userID)
	if err != nil {
		return nil, err
	}
	if hasSuperuser {
		sources.Add("superuser", "system:root")
		return &model.PermissionCheckResult{
			HasPermission:  true,
			PermissionType: permissionType,
			Sources:        sourcesToStrings(sources),
			DurationMs:     float64(time.Since(startTime).Milliseconds()),
		}, nil
	}

	// Path 2: Direct permission (single query)
	hasDirect, err := r.checkDirectPermission(ctx, userID, documentID, permissionType, &sources)
	if err != nil {
		return nil, err
	}
	if hasDirect {
		return &model.PermissionCheckResult{
			HasPermission:  true,
			PermissionType: permissionType,
			Sources:        sourcesToStrings(sources),
			DurationMs:     float64(time.Since(startTime).Milliseconds()),
		}, nil
	}

	// Path 3: Customer follower permission (2 queries)
	hasCustomer, err := r.checkCustomerFollowerPermission(ctx, userID, documentID, permissionType, &sources)
	if err != nil {
		return nil, err
	}
	if hasCustomer {
		return &model.PermissionCheckResult{
			HasPermission:  true,
			PermissionType: permissionType,
			Sources:        sourcesToStrings(sources),
			DurationMs:     float64(time.Since(startTime).Milliseconds()),
		}, nil
	}

	// Path 4: Manager chain permission - OPTIMIZED with forward expansion
	// Instead of checking "who has access to this document and am I their manager",
	// we check "what documents can my subordinates access and is this document in that set"
	hasManager, err := r.checkManagerChainPermissionOptimized(ctx, userID, documentID, permissionType, &sources)
	if err != nil {
		return nil, err
	}
	if hasManager {
		return &model.PermissionCheckResult{
			HasPermission:  true,
			PermissionType: permissionType,
			Sources:        sourcesToStrings(sources),
			DurationMs:     float64(time.Since(startTime).Milliseconds()),
		}, nil
	}

	// No permission found
	return &model.PermissionCheckResult{
		HasPermission: false,
		DurationMs:    float64(time.Since(startTime).Milliseconds()),
	}, nil
}

// checkDirectPermission checks if user has direct permission on document
func (r *ZanzibarPermissionRepository) checkDirectPermission(ctx context.Context, userID, documentID, permissionType string, sources *model.PermissionSourceList) (bool, error) {
	var tuple model.RelationTuple
	err := r.db.WithContext(ctx).
		Where("namespace = ? AND object_id = ? AND relation = ? AND subject_namespace = ? AND subject_id = ?",
			"document", documentID, permissionType, "user", userID).
		First(&tuple).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}

	sources.Add("direct", documentID)
	return true, nil
}

// checkCustomerFollowerPermission checks if user has permission through customer follower relationship
func (r *ZanzibarPermissionRepository) checkCustomerFollowerPermission(ctx context.Context, userID, documentID, permissionType string, sources *model.PermissionSourceList) (bool, error) {
	// Step 1: Find which customer owns this document
	var ownerTuple model.RelationTuple
	err := r.db.WithContext(ctx).
		Where("namespace = ? AND object_id = ? AND relation = ?", "document", documentID, "owner_customer").
		First(&ownerTuple).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}

	customerID := ownerTuple.SubjectID

	// Step 2: Check if user is a follower of this customer
	var followerTuple model.RelationTuple
	err = r.db.WithContext(ctx).
		Where("namespace = ? AND object_id = ? AND relation = ? AND subject_namespace = ? AND subject_id = ?",
			"customer", customerID, "follower", "user", userID).
		First(&followerTuple).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}

	sources.Add("customer_follower", customerID)
	return true, nil
}

// checkManagerChainPermission checks if user has permission through manager chain
// Business logic (OR/Union of two conditions):
//  1. User manages any customer follower who has access to the document
//  2. User manages the document creator
//
// Either condition being true grants permission.
func (r *ZanzibarPermissionRepository) checkManagerChainPermission(ctx context.Context, userID, documentID, permissionType string, sources *model.PermissionSourceList) (bool, error) {
	// Condition 1: Check if user manages any customer follower
	// Step 1.1: Find the customer that owns this document
	var docCustomerTuple model.RelationTuple
	err := r.db.WithContext(ctx).
		Where("namespace = ? AND object_id = ? AND relation = ?", "document", documentID, "owner_customer").
		First(&docCustomerTuple).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return false, err
	}

	if err == nil {
		customerID := docCustomerTuple.SubjectID

		// Step 1.2: Find all followers of this customer
		var followerTuples []model.RelationTuple
		err = r.db.WithContext(ctx).
			Where("namespace = ? AND object_id = ? AND relation = ?", "customer", customerID, "follower").
			Find(&followerTuples).Error

		if err != nil {
			return false, err
		}

		// Step 1.3: For each follower, check if current user is their manager
		for _, tuple := range followerTuples {
			if tuple.SubjectNamespace != "user" {
				continue
			}

			followerID := tuple.SubjectID

			isManager, err := r.isInManagementChain(ctx, userID, followerID, make(map[string]bool), 0, 5)
			if err != nil {
				return false, err
			}

			if isManager {
				sources.Add("manager_of_follower", followerID)
				return true, nil // Condition 1 satisfied
			}
		}
	}

	// Condition 2: Check if user manages the document creator
	var creatorTuple model.RelationTuple
	err = r.db.WithContext(ctx).
		Where("namespace = ? AND object_id = ? AND relation = ?", "document", documentID, "owner").
		First(&creatorTuple).Error

	if err == nil && creatorTuple.SubjectNamespace == "user" {
		isManager, err := r.isInManagementChain(ctx, userID, creatorTuple.SubjectID, make(map[string]bool), 0, 5)
		if err != nil {
			return false, err
		}
		if isManager {
			sources.Add("manager_of_creator", creatorTuple.SubjectID)
			return true, nil // Condition 2 satisfied
		}
	}

	return false, nil // Neither condition satisfied
}

// checkManagerChainPermissionOptimized uses forward expansion strategy
// Instead of: "find all followers of document's customer, check if I manage any of them" (slow)
// We do: "find all my subordinates, check if any of them have access to this document" (fast)
func (r *ZanzibarPermissionRepository) checkManagerChainPermissionOptimized(ctx context.Context, userID, documentID, permissionType string, sources *model.PermissionSourceList) (bool, error) {
	// Step 1: Get all subordinates of the current user (batch query)
	subordinateIDs, err := r.getAllSubordinates(ctx, userID, 5)
	if err != nil {
		return false, err
	}

	if len(subordinateIDs) == 0 {
		return false, nil
	}

	// Step 2: Check if any subordinate is the document owner
	var ownerCount int64
	err = r.db.WithContext(ctx).Model(&model.RelationTuple{}).
		Where("namespace = ? AND object_id = ? AND relation = ? AND subject_namespace = ? AND subject_id IN ?",
			"document", documentID, "owner", "user", subordinateIDs).
		Count(&ownerCount).Error
	if err != nil {
		return false, err
	}
	if ownerCount > 0 {
		sources.Add("manager_of_creator", "subordinate")
		return true, nil
	}

	// Step 3: Check if any subordinate follows the document's customer
	// First get document's customer
	var docCustomerTuple model.RelationTuple
	err = r.db.WithContext(ctx).
		Where("namespace = ? AND object_id = ? AND relation = ?", "document", documentID, "owner_customer").
		First(&docCustomerTuple).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}

	// Check if any subordinate follows this customer
	var followerCount int64
	err = r.db.WithContext(ctx).Model(&model.RelationTuple{}).
		Where("namespace = ? AND object_id = ? AND relation = ? AND subject_namespace = ? AND subject_id IN ?",
			"customer", docCustomerTuple.SubjectID, "follower", "user", subordinateIDs).
		Count(&followerCount).Error
	if err != nil {
		return false, err
	}
	if followerCount > 0 {
		sources.Add("manager_of_follower", "subordinate")
		return true, nil
	}

	return false, nil
}

// isInManagementChain checks if managerUserID is in the management chain of subordinateUserID
// Uses depth-limited recursion to prevent infinite loops
func (r *ZanzibarPermissionRepository) isInManagementChain(ctx context.Context, managerUserID, subordinateUserID string, visited map[string]bool, currentDepth, maxDepth int) (bool, error) {
	if currentDepth > maxDepth {
		return false, nil
	}

	// Prevent cycles
	if visited[subordinateUserID] {
		return false, nil
	}
	visited[subordinateUserID] = true

	// Find all managers for this user
	var managerRelations []model.ManagementRelation
	err := r.db.WithContext(ctx).
		Joins("JOIN user_departments ON user_departments.department_id = management_relations.department_id").
		Where("management_relations.subordinate_user_id = ? AND user_departments.user_id = ?", subordinateUserID, subordinateUserID).
		Find(&managerRelations).Error

	if err != nil {
		return false, err
	}

	// Check if managerUserID is a direct manager
	for _, rel := range managerRelations {
		if rel.ManagerUserID == managerUserID {
			return true, nil
		}
	}

	// For each manager, recursively check if they're in the chain
	for _, rel := range managerRelations {
		isInChain, err := r.isInManagementChain(ctx, managerUserID, rel.ManagerUserID, visited, currentDepth+1, maxDepth)
		if err != nil {
			return false, err
		}

		if isInChain {
			return true, nil
		}
	}

	return false, nil
}

// checkSuperuserPermission checks if user is a superuser
func (r *ZanzibarPermissionRepository) checkSuperuserPermission(ctx context.Context, userID string) (bool, error) {
	var tuple model.RelationTuple
	err := r.db.WithContext(ctx).
		Where("namespace = ? AND object_id = ? AND relation = ? AND subject_namespace = ? AND subject_id = ?",
			"system", "root", "admin", "user", userID).
		First(&tuple).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// CheckPermissionsBatch checks permissions for multiple documents
func (r *ZanzibarPermissionRepository) CheckPermissionsBatch(ctx context.Context, userID string, documentIDs []string, permissionType string) (map[string]bool, error) {
	result := make(map[string]bool)
	for _, docID := range documentIDs {
		result[docID] = false
	}

	if len(documentIDs) == 0 {
		return result, nil
	}

	// Path 1: Superuser check
	isSuperuser, err := r.checkSuperuserPermission(ctx, userID)
	if err != nil {
		return nil, err
	}
	if isSuperuser {
		for _, docID := range documentIDs {
			result[docID] = true
		}
		return result, nil
	}

	// Path 2: Direct permissions
	var directDocIDs []string
	err = r.db.WithContext(ctx).Model(&model.RelationTuple{}).
		Where("namespace = ? AND object_id IN ? AND relation = ? AND subject_namespace = ? AND subject_id = ?",
			"document", documentIDs, permissionType, "user", userID).
		Pluck("object_id", &directDocIDs).Error
	if err != nil {
		return nil, err
	}
	for _, docID := range directDocIDs {
		result[docID] = true
	}

	// Path 3: Customer follower permissions
	// 3.1 Find customers user follows
	var followedCustomerIDs []string
	err = r.db.WithContext(ctx).Model(&model.RelationTuple{}).
		Where("namespace = ? AND relation = ? AND subject_namespace = ? AND subject_id = ?",
			"customer", "follower", "user", userID).
		Pluck("object_id", &followedCustomerIDs).Error
	if err != nil {
		return nil, err
	}

	if len(followedCustomerIDs) > 0 {
		var customerDocIDs []string
		err = r.db.WithContext(ctx).Model(&model.RelationTuple{}).
			Where("namespace = ? AND object_id IN ? AND relation = ? AND subject_id IN ?",
				"document", documentIDs, "owner_customer", followedCustomerIDs).
			Pluck("object_id", &customerDocIDs).Error
		if err != nil {
			return nil, err
		}
		for _, docID := range customerDocIDs {
			result[docID] = true
		}
	}

	// Path 4: Manager chain permissions
	subordinateIDs, err := r.getAllSubordinates(ctx, userID, 5)
	if err != nil {
		return nil, err
	}

	if len(subordinateIDs) > 0 {
		// 4.1 Subordinate is owner
		var subOwnerDocIDs []string
		err = r.db.WithContext(ctx).Model(&model.RelationTuple{}).
			Where("namespace = ? AND object_id IN ? AND relation = ? AND subject_namespace = ? AND subject_id IN ?",
				"document", documentIDs, "owner", "user", subordinateIDs).
			Pluck("object_id", &subOwnerDocIDs).Error
		if err != nil {
			return nil, err
		}
		for _, docID := range subOwnerDocIDs {
			result[docID] = true
		}

		// 4.2 Subordinate follows owner customer
		var subFollowedCustomerIDs []string
		err = r.db.WithContext(ctx).Model(&model.RelationTuple{}).
			Where("namespace = ? AND relation = ? AND subject_namespace = ? AND subject_id IN ?",
				"customer", "follower", "user", subordinateIDs).
			Pluck("object_id", &subFollowedCustomerIDs).Error
		if err != nil {
			return nil, err
		}

		if len(subFollowedCustomerIDs) > 0 {
			var subCustomerDocIDs []string
			err = r.db.WithContext(ctx).Model(&model.RelationTuple{}).
				Where("namespace = ? AND object_id IN ? AND relation = ? AND subject_id IN ?",
					"document", documentIDs, "owner_customer", subFollowedCustomerIDs).
				Pluck("object_id", &subCustomerDocIDs).Error
			if err != nil {
				return nil, err
			}
			for _, docID := range subCustomerDocIDs {
				result[docID] = true
			}
		}
	}

	return result, nil
}

// GetUserDocuments returns paginated list of documents user can access
// This is more complex for Zanzibar as we need to traverse the graph
func (r *ZanzibarPermissionRepository) GetUserDocuments(ctx context.Context, userID string, permissionType string, page, pageSize int) (*model.UserDocumentList, error) {
	startTime := time.Now()
	offset := (page - 1) * pageSize

	// Strategy: Expand user's identity first, then query documents
	// 1. Superuser check (has access to ALL documents)
	// 2. Direct document permissions
	// 3. Customer follower permissions
	// 4. Manager chain permissions (subordinates' documents)

	// Path 0: Check if user is superuser
	isSuperuser, err := r.checkSuperuserPermission(ctx, userID)
	if err != nil {
		return nil, err
	}

	if isSuperuser {
		// Superuser has access to ALL documents
		var total int64
		r.db.WithContext(ctx).Model(&model.Document{}).Count(&total)

		var documents []model.Document
		err = r.db.WithContext(ctx).
			Preload("Customer").
			Preload("Creator").
			Order("created_at DESC").
			Limit(pageSize).
			Offset(offset).
			Find(&documents).Error

		if err != nil {
			return nil, fmt.Errorf("failed to fetch documents for superuser: %w", err)
		}

		documentItems := make([]model.DocumentListItem, 0, len(documents))
		for _, doc := range documents {
			docItem := model.DocumentListItem{
				ID:             doc.ID,
				Title:          doc.Title,
				CustomerID:     doc.CustomerID,
				CreatorID:      doc.CreatorID,
				PermissionType: permissionType,
				SourceType:     "superuser",
				CreatedAt:      doc.CreatedAt,
			}
			if doc.Customer != nil {
				docItem.CustomerName = doc.Customer.Name
			}
			if doc.Creator != nil {
				docItem.CreatorName = doc.Creator.Name
			}
			documentItems = append(documentItems, docItem)
		}

		return &model.UserDocumentList{
			Documents:  documentItems,
			Total:      total,
			Page:       page,
			PageSize:   pageSize,
			DurationMs: float64(time.Since(startTime).Milliseconds()),
		}, nil
	}

	var documentIDs []string

	// Path 1: Direct permissions
	r.db.WithContext(ctx).
		Where("namespace = ? AND relation = ? AND subject_namespace = ? AND subject_id = ?",
			"document", permissionType, "user", userID).
		Pluck("object_id", &documentIDs)

	// Path 2: Customer follower permissions
	var customerIDs []string
	r.db.WithContext(ctx).
		Where("namespace = ? AND relation = ? AND subject_namespace = ? AND subject_id = ?",
			"customer", "follower", "user", userID).
		Pluck("object_id", &customerIDs)

	if len(customerIDs) > 0 {
		var customerDocIDs []string
		r.db.WithContext(ctx).
			Where("namespace = ? AND relation = ? AND subject_namespace = ? AND subject_id IN ?",
				"document", "owner_customer", "customer", customerIDs).
			Pluck("object_id", &customerDocIDs)
		documentIDs = append(documentIDs, customerDocIDs...)
	}

	// Path 3: Manager chain permissions (documents accessible by subordinates)
	// Find all subordinates
	subordinateIDs, err := r.getAllSubordinates(ctx, userID, 5)
	if err != nil {
		return nil, err
	}

	if len(subordinateIDs) > 0 {
		// 3.1: Documents directly owned by subordinates
		var subordinateDocIDs []string
		r.db.WithContext(ctx).
			Where("namespace = ? AND relation = ? AND subject_namespace = ? AND subject_id IN ?",
				"document", "owner", "user", subordinateIDs).
			Pluck("object_id", &subordinateDocIDs)
		documentIDs = append(documentIDs, subordinateDocIDs...)

		// 3.2: Documents accessible via subordinates' customer followings
		var subordinateCustomerIDs []string
		r.db.WithContext(ctx).
			Where("namespace = ? AND relation = ? AND subject_namespace = ? AND subject_id IN ?",
				"customer", "follower", "user", subordinateIDs).
			Pluck("object_id", &subordinateCustomerIDs)

		if len(subordinateCustomerIDs) > 0 {
			var subordinateCustomerDocIDs []string
			r.db.WithContext(ctx).
				Where("namespace = ? AND relation = ? AND subject_namespace = ? AND subject_id IN ?",
					"document", "owner_customer", "customer", subordinateCustomerIDs).
				Pluck("object_id", &subordinateCustomerDocIDs)
			documentIDs = append(documentIDs, subordinateCustomerDocIDs...)
		}
	}

	// Deduplicate
	uniqueDocIDs := make([]string, 0, len(documentIDs))
	seen := make(map[string]bool)
	for _, id := range documentIDs {
		if !seen[id] {
			seen[id] = true
			uniqueDocIDs = append(uniqueDocIDs, id)
		}
	}

	total := int64(len(uniqueDocIDs))

	// Fetch documents with pagination
	var documents []model.Document
	err = r.db.WithContext(ctx).
		Where("id IN ?", uniqueDocIDs).
		Preload("Customer").
		Preload("Creator").
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&documents).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch user documents: %w", err)
	}

	// Convert to document list items
	documentItems := make([]model.DocumentListItem, 0, len(documents))
	for _, doc := range documents {
		// Determine permission source
		var sourceType string
		if r.hasDirectPermission(ctx, userID, doc.ID, permissionType) {
			sourceType = "direct"
		} else if r.hasCustomerPermission(ctx, userID, doc.ID, permissionType) {
			sourceType = "customer_follower"
		} else {
			sourceType = "manager_chain"
		}

		docItem := model.DocumentListItem{
			ID:             doc.ID,
			Title:          doc.Title,
			CustomerID:     doc.CustomerID,
			CreatorID:      doc.CreatorID,
			PermissionType: permissionType,
			SourceType:     sourceType,
			CreatedAt:      doc.CreatedAt,
		}

		if doc.Customer != nil {
			docItem.CustomerName = doc.Customer.Name
		}
		if doc.Creator != nil {
			docItem.CreatorName = doc.Creator.Name
		}

		documentItems = append(documentItems, docItem)
	}

	duration := time.Since(startTime).Milliseconds()

	return &model.UserDocumentList{
		Documents:  documentItems,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		DurationMs: float64(duration),
	}, nil
}

// getAllSubordinates gets all subordinates of a manager using RelationTuple with BFS to avoid N+1 queries
// This implements the Zanzibar way: department#manager manages department#member
func (r *ZanzibarPermissionRepository) getAllSubordinates(ctx context.Context, managerUserID string, maxDepth int) ([]string, error) {
	allSubordinateIDs := make([]string, 0)
	visited := map[string]bool{managerUserID: true}
	currentManagers := []string{managerUserID}

	for depth := 0; depth < maxDepth; depth++ {
		if len(currentManagers) == 0 {
			break
		}

		// Step 1: Find all departments where these users are managers
		var managedDeptIDs []string
		err := r.db.WithContext(ctx).Model(&model.RelationTuple{}).
			Where("namespace = ? AND relation = ? AND subject_namespace = ? AND subject_id IN ?",
				"department", "manager", "user", currentManagers).
			Pluck("object_id", &managedDeptIDs).Error

		if err != nil {
			return nil, err
		}

		if len(managedDeptIDs) == 0 {
			break
		}

		// Step 2: Find all members of these departments
		var memberIDs []string
		err = r.db.WithContext(ctx).Model(&model.RelationTuple{}).
			Where("namespace = ? AND object_id IN ? AND relation = ? AND subject_namespace = ?",
				"department", managedDeptIDs, "member", "user").
			Pluck("subject_id", &memberIDs).Error

		if err != nil {
			return nil, err
		}

		// Step 3: Filter out already visited members and prepare for next level
		nextManagers := make([]string, 0)
		for _, id := range memberIDs {
			if !visited[id] {
				visited[id] = true
				allSubordinateIDs = append(allSubordinateIDs, id)
				nextManagers = append(nextManagers, id)
			}
		}
		currentManagers = nextManagers
	}

	return allSubordinateIDs, nil
}

// hasDirectPermission helper for GetUserDocuments
func (r *ZanzibarPermissionRepository) hasDirectPermission(ctx context.Context, userID, documentID, permissionType string) bool {
	var tuple model.RelationTuple
	err := r.db.WithContext(ctx).
		Where("namespace = ? AND object_id = ? AND relation = ? AND subject_namespace = ? AND subject_id = ?",
			"document", documentID, permissionType, "user", userID).
		First(&tuple).Error
	return err == nil
}

// hasCustomerPermission helper for GetUserDocuments
func (r *ZanzibarPermissionRepository) hasCustomerPermission(ctx context.Context, userID, documentID, permissionType string) bool {
	var ownerTuple model.RelationTuple
	err := r.db.WithContext(ctx).
		Where("namespace = ? AND object_id = ? AND relation = ?", "document", documentID, "owner_customer").
		First(&ownerTuple).Error
	if err != nil {
		return false
	}

	var followerTuple model.RelationTuple
	err = r.db.WithContext(ctx).
		Where("namespace = ? AND object_id = ? AND relation = ? AND subject_namespace = ? AND subject_id = ?",
			"customer", ownerTuple.SubjectID, "follower", "user", userID).
		First(&followerTuple).Error
	return err == nil
}

// GrantDirectPermission grants direct permission using tuple
func (r *ZanzibarPermissionRepository) GrantDirectPermission(ctx context.Context, userID, documentID, permissionType string) error {
	tuple := &model.RelationTuple{
		Namespace:        "document",
		ObjectID:         documentID,
		Relation:         permissionType,
		SubjectNamespace: "user",
		SubjectID:        userID,
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			DoNothing: true,
		}).
		Create(tuple).Error
}

// RevokePermission revokes permission by deleting tuple
func (r *ZanzibarPermissionRepository) RevokePermission(ctx context.Context, userID, documentID string) error {
	return r.db.WithContext(ctx).
		Where("namespace = ? AND object_id = ? AND subject_namespace = ? AND subject_id = ?",
			"document", documentID, "user", userID).
		Delete(&model.RelationTuple{}).Error
}

// AddCustomerFollower adds a follower tuple to customer
func (r *ZanzibarPermissionRepository) AddCustomerFollower(ctx context.Context, customerID, userID string) error {
	tuple := &model.RelationTuple{
		Namespace:        "customer",
		ObjectID:         customerID,
		Relation:         "follower",
		SubjectNamespace: "user",
		SubjectID:        userID,
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			DoNothing: true,
		}).
		Create(tuple).Error
}

// RemoveCustomerFollower removes a follower from customer
func (r *ZanzibarPermissionRepository) RemoveCustomerFollower(ctx context.Context, customerID, userID string) error {
	return r.db.WithContext(ctx).
		Where("namespace = ? AND object_id = ? AND relation = ? AND subject_namespace = ? AND subject_id = ?",
			"customer", customerID, "follower", "user", userID).
		Delete(&model.RelationTuple{}).Error
}

// UpdateDepartmentManager updates department manager - SINGLE TUPLE UPDATE!
func (r *ZanzibarPermissionRepository) UpdateDepartmentManager(ctx context.Context, departmentID, newManagerID string) error {
	// Delete old manager tuple
	r.db.WithContext(ctx).
		Where("namespace = ? AND object_id = ? AND relation = ?", "department", departmentID, "manager").
		Delete(&model.RelationTuple{})

	// Add new manager tuple
	tuple := &model.RelationTuple{
		Namespace:        "department",
		ObjectID:         departmentID,
		Relation:         "manager",
		SubjectNamespace: "user",
		SubjectID:        newManagerID,
	}

	return r.db.WithContext(ctx).Create(tuple).Error
}

// AddUserToDepartment adds user to department
func (r *ZanzibarPermissionRepository) AddUserToDepartment(ctx context.Context, userID, departmentID, role string, isPrimary bool) error {
	// Add to user_departments table
	userDept := &model.UserDepartment{
		UserID:       userID,
		DepartmentID: departmentID,
		Role:         role,
		IsPrimary:    isPrimary,
	}

	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			DoNothing: true,
		}).
		Create(userDept).Error; err != nil {
		return err
	}

	// Add membership tuple
	tuple := &model.RelationTuple{
		Namespace:        "department",
		ObjectID:         departmentID,
		Relation:         "member",
		SubjectNamespace: "user",
		SubjectID:        userID,
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			DoNothing: true,
		}).
		Create(tuple).Error
}

// RemoveUserFromDepartment removes user from department
func (r *ZanzibarPermissionRepository) RemoveUserFromDepartment(ctx context.Context, userID, departmentID string) error {
	// Delete from user_departments table
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND department_id = ?", userID, departmentID).
		Delete(&model.UserDepartment{}).Error; err != nil {
		return err
	}

	// Delete membership tuple
	return r.db.WithContext(ctx).
		Where("namespace = ? AND object_id = ? AND relation = ? AND subject_namespace = ? AND subject_id = ?",
			"department", departmentID, "member", "user", userID).
		Delete(&model.RelationTuple{}).Error
}

// GetStorageStats returns storage statistics for Zanzibar tuples
func (r *ZanzibarPermissionRepository) GetStorageStats(ctx context.Context) (*model.StorageStats, error) {
	var stats model.StorageStats

	err := r.db.WithContext(ctx).
		Raw(`
			SELECT
				'Zanzibar' as engine_type,
				'relation_tuples' as table_name,
				TABLE_ROWS as row_count,
				ROUND(DATA_LENGTH / 1024 / 1024, 2) as data_size_mb,
				ROUND(INDEX_LENGTH / 1024 / 1024, 2) as index_size_mb,
				ROUND((DATA_LENGTH + INDEX_LENGTH) / 1024 / 1024, 2) as total_size_mb
			FROM information_schema.TABLES
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'relation_tuples'
		`).
		Scan(&stats).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get storage stats: %w", err)
	}

	return &stats, nil
}

// GetTupleStats returns tuple statistics by namespace and relation
func (r *ZanzibarPermissionRepository) GetTupleStats(ctx context.Context) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	err := r.db.WithContext(ctx).
		Model(&model.RelationTuple{}).
		Select("namespace, relation, COUNT(*) as total_tuples, COUNT(DISTINCT object_id) as unique_objects, COUNT(DISTINCT subject_id) as unique_subjects").
		Group("namespace, relation").
		Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get tuple stats: %w", err)
	}

	return results, nil
}

// sourcesToStrings converts PermissionSourceList to []string
func sourcesToStrings(sources model.PermissionSourceList) []string {
	result := make([]string, len(sources))
	for i, source := range sources {
		result[i] = fmt.Sprintf("%s:%s", source.Type, source.SourceID)
	}
	return result
}

// GrantSuperuser grants superuser privileges to a user
func (r *ZanzibarPermissionRepository) GrantSuperuser(ctx context.Context, userID string) error {
	// Add superuser tuple
	tuple := model.RelationTuple{
		Namespace:        "system",
		ObjectID:         "root",
		Relation:         "admin",
		SubjectNamespace: "user",
		SubjectID:        userID,
	}

	return r.db.WithContext(ctx).Create(&tuple).Error
}

// RevokeSuperuser revokes superuser privileges from a user
func (r *ZanzibarPermissionRepository) RevokeSuperuser(ctx context.Context, userID string) error {
	// Remove superuser tuple
	return r.db.WithContext(ctx).
		Where("namespace = ? AND object_id = ? AND relation = ? AND subject_namespace = ? AND subject_id = ?",
			"system", "root", "admin", "user", userID).
		Delete(&model.RelationTuple{}).Error
}
