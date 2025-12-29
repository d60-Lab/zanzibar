package model

import (
	"time"
)

// =====================================================
// Business Entities
// =====================================================

// User represents a user in the system
type User struct {
	ID                 string     `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Name               string     `gorm:"type:varchar(100);not null" json:"name"`
	Email              string     `gorm:"type:varchar(100);uniqueIndex;not null" json:"email"`
	PrimaryDepartmentID *string   `gorm:"type:varchar(36)" json:"primary_department_id,omitempty"`
	IsSuperuser        bool       `gorm:"default:false" json:"is_superuser"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	DeletedAt          *time.Time `gorm:"index" json:"deleted_at,omitempty"`

	// Relations
	PrimaryDepartment *Department         `gorm:"foreignKey:PrimaryDepartmentID" json:"primary_department,omitempty"`
	Departments       []UserDepartment    `gorm:"foreignKey:UserID" json:"departments,omitempty"`
	CreatedDocuments  []Document          `gorm:"foreignKey:CreatorID" json:"created_documents,omitempty"`
}

// Department represents an organizational department
type Department struct {
	ID        string     `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Name      string     `gorm:"type:varchar(100);not null" json:"name"`
	ParentID  *string    `gorm:"type:varchar(36);index" json:"parent_id,omitempty"`
	Level     int        `gorm:"not null;check:level >= 1 AND level <= 5" json:"level"`
	ManagerID *string    `gorm:"type:varchar(36);index" json:"manager_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	// Relations
	Parent   *Department       `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Manager  *User             `gorm:"foreignKey:ManagerID" json:"manager,omitempty"`
	Children []Department      `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	Members  []UserDepartment  `gorm:"foreignKey:DepartmentID" json:"members,omitempty"`
}

// UserDepartment represents the many-to-many relationship between users and departments
type UserDepartment struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       string    `gorm:"type:varchar(36);not null;uniqueIndex:uk_user_dept" json:"user_id"`
	DepartmentID string    `gorm:"type:varchar(36);not null;uniqueIndex:uk_user_dept" json:"department_id"`
	Role         string    `gorm:"type:varchar(20);not null;check:role IN ('member','leader','director')" json:"role"` // member, leader, director
	IsPrimary    bool      `gorm:"default:false" json:"is_primary"`
	CreatedAt    time.Time `json:"created_at"`

	// Relations
	User       *User       `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Department *Department `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
}

// ManagementRelation represents a management relationship between users
type ManagementRelation struct {
	ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ManagerUserID    string    `gorm:"type:varchar(36);not null;uniqueIndex:uk_manager_subordinate_dept" json:"manager_user_id"`
	SubordinateUserID string   `gorm:"type:varchar(36);not null;uniqueIndex:uk_manager_subordinate_dept" json:"subordinate_user_id"`
	DepartmentID     string    `gorm:"type:varchar(36);not null;uniqueIndex:uk_manager_subordinate_dept" json:"department_id"`
	ManagementLevel  int       `gorm:"not null;check:management_level >= 1 AND management_level <= 5" json:"management_level"`
	CreatedAt        time.Time `json:"created_at"`

	// Relations
	Manager    *User       `gorm:"foreignKey:ManagerUserID" json:"manager,omitempty"`
	Subordinate *User      `gorm:"foreignKey:SubordinateUserID" json:"subordinate,omitempty"`
	Department *Department `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
}

// Customer represents a customer in the system
type Customer struct {
	ID        string     `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Name      string     `gorm:"type:varchar(100);not null" json:"name"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`

	// Relations
	Followers []CustomerFollower `gorm:"foreignKey:CustomerID" json:"followers,omitempty"`
	Documents []Document         `gorm:"foreignKey:CustomerID" json:"documents,omitempty"`
}

// CustomerFollower represents users following a customer
type CustomerFollower struct {
	CustomerID string    `gorm:"type:varchar(36);not null;primaryKey" json:"customer_id"`
	UserID     string    `gorm:"type:varchar(36);not null;primaryKey;index" json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`

	// Relations
	Customer *Customer `gorm:"foreignKey:CustomerID" json:"customer,omitempty"`
	User     *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// Document represents a document in the system
type Document struct {
	ID        string     `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Title     string     `gorm:"type:varchar(200);not null" json:"title"`
	CustomerID string    `gorm:"type:varchar(36);not null;index" json:"customer_id"`
	CreatorID string    `gorm:"type:varchar(36);not null;index" json:"creator_id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`

	// Relations
	Customer *Customer `gorm:"foreignKey:CustomerID" json:"customer,omitempty"`
	Creator  *User     `gorm:"foreignKey:CreatorID" json:"creator,omitempty"`
}

// =====================================================
// MySQL Permission Model (Expanded Storage)
// =====================================================

// DocumentPermissionMySQL represents a pre-computed permission row
type DocumentPermissionMySQL struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         string    `gorm:"type:varchar(36);not null;uniqueIndex:uk_user_doc" json:"user_id"`
	DocumentID     string    `gorm:"type:varchar(36);not null;uniqueIndex:uk_user_doc" json:"document_id"`
	PermissionType string    `gorm:"type:enum('viewer','editor','owner');not null;uniqueIndex:uk_user_doc" json:"permission_type"`
	SourceType     string    `gorm:"type:enum('direct','customer_follower','manager_chain','superuser');not null" json:"source_type"`
	SourceID       *string   `gorm:"type:varchar(36)" json:"source_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	// Relations
	User     *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Document *Document `gorm:"foreignKey:DocumentID" json:"document,omitempty"`
}

// =====================================================
// Zanzibar Permission Model (Tuple-Based)
// =====================================================

// RelationTuple represents a Zanzibar-style relation tuple
type RelationTuple struct {
	ID               int64      `gorm:"primaryKey;autoIncrement" json:"id"`

	// Object: what we're protecting
	Namespace        string     `gorm:"type:varchar(50);not null;uniqueIndex:uk_tuple" json:"namespace"`
	ObjectID         string     `gorm:"type:varchar(36);not null;uniqueIndex:uk_tuple" json:"object_id"`
	Relation         string     `gorm:"type:varchar(50);not null;uniqueIndex:uk_tuple" json:"relation"`

	// Subject: who has access
	SubjectNamespace string     `gorm:"type:varchar(50);not null;uniqueIndex:uk_tuple" json:"subject_namespace"`
	SubjectID        string     `gorm:"type:varchar(36);not null;uniqueIndex:uk_tuple" json:"subject_id"`

	// For computed/union relations (advanced Zanzibar feature)
	UsersetNamespace *string    `gorm:"type:varchar(50)" json:"userset_namespace,omitempty"`
	UsersetRelation  *string    `gorm:"type:varchar(50)" json:"userset_relation,omitempty"`

	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// TupleString returns the string representation of the tuple (Zanzibar format)
func (t *RelationTuple) TupleString() string {
	if t.UsersetNamespace != nil && t.UsersetRelation != nil {
		// Computed relation: namespace:object_id#relation@userset_namespace:userset_relation
		return t.Namespace + ":" + t.ObjectID + "#" + t.Relation + "@" + *t.UsersetNamespace + ":" + *t.UsersetRelation
	}
	// Direct relation: namespace:object_id#relation@subject_namespace:subject_id
	return t.Namespace + ":" + t.ObjectID + "#" + t.Relation + "@" + t.SubjectNamespace + ":" + t.SubjectID
}

// =====================================================
// Benchmark Models
// =====================================================

// BenchmarkLog represents a benchmark execution log
type BenchmarkLog struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TestName      string    `gorm:"type:varchar(100);not null;index" json:"test_name"`
	EngineType    string    `gorm:"type:enum('mysql','zanzibar');not null;index" json:"engine_type"`
	OperationType string    `gorm:"type:varchar(50);not null;index" json:"operation_type"`
	DurationMs    float64   `gorm:"type:decimal(10,3);not null" json:"duration_ms"`
	RowsAffected  int       `gorm:"default:0" json:"rows_affected"`
	CacheHit      bool      `gorm:"default:false" json:"cache_hit"`
	ErrorMessage  *string   `gorm:"type:text" json:"error_message,omitempty"`
	Metadata      *string   `gorm:"type:json" json:"metadata,omitempty"`
	CreatedAt     time.Time `json:"created_at"`

	// Relations
	Metrics []BenchmarkMetric `gorm:"foreignKey:BenchmarkID" json:"metrics,omitempty"`
}

// BenchmarkMetric represents detailed metrics for a benchmark run
type BenchmarkMetric struct {
	ID          int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	BenchmarkID int64   `gorm:"not null;index" json:"benchmark_id"`
	MetricType  string  `gorm:"type:varchar(50);not null;index" json:"metric_type"`  // cpu, memory, io, etc.
	MetricName  string  `gorm:"type:varchar(100);not null" json:"metric_name"`
	MetricValue float64 `gorm:"type:decimal(15,3);not null" json:"metric_value"`
	Unit        string  `gorm:"type:varchar(20);not null" json:"unit"`
	CreatedAt   time.Time `json:"created_at"`

	// Relations
	Benchmark *BenchmarkLog `gorm:"foreignKey:BenchmarkID" json:"benchmark,omitempty"`
}

// =====================================================
// DTOs and Helper Types
// =====================================================

// PermissionCheckResult represents the result of a permission check
type PermissionCheckResult struct {
	HasPermission bool          `json:"has_permission"`
	PermissionType string       `json:"permission_type,omitempty"`
	Sources       []string      `json:"sources,omitempty"` // Where the permission came from
	CacheHit      bool          `json:"cache_hit"`
	DurationMs    float64       `json:"duration_ms"`
}

// UserDocumentList represents a paginated list of documents a user can access
type UserDocumentList struct {
	Documents []DocumentListItem `json:"documents"`
	Total     int64              `json:"total"`
	Page      int                `json:"page"`
	PageSize  int                `json:"page_size"`
	DurationMs float64           `json:"duration_ms"`
}

// DocumentListItem represents a document in a list
type DocumentListItem struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	CustomerID     string    `json:"customer_id"`
	CustomerName   string    `json:"customer_name,omitempty"`
	CreatorID      string    `json:"creator_id"`
	CreatorName    string    `json:"creator_name,omitempty"`
	PermissionType string    `json:"permission_type"`
	SourceType     string    `json:"source_type"`
	CreatedAt      time.Time `json:"created_at"`
}

// StorageStats represents storage statistics for comparison
type StorageStats struct {
	EngineType   string  `json:"engine_type"`
	TableName    string  `json:"table_name"`
	RowCount     int64   `json:"row_count"`
	DataSizeMB   float64 `json:"data_size_mb"`
	IndexSizeMB  float64 `json:"index_size_mb"`
	TotalSizeMB  float64 `json:"total_size_mb"`
}

// PermissionSource represents where a permission originated from
type PermissionSource struct {
	Type     string `json:"type"`     // direct, customer_follower, manager_chain, superuser
	SourceID string `json:"source_id"` // ID of the source entity
}

// PermissionSourceList is a list of PermissionSource
type PermissionSourceList []PermissionSource

// Add appends a new permission source
func (l *PermissionSourceList) Add(sourceType, sourceID string) {
	*l = append(*l, PermissionSource{
		Type:     sourceType,
		SourceID: sourceID,
	})
}

// Contains checks if a permission source already exists in the list
func (l PermissionSourceList) Contains(sourceType, sourceID string) bool {
	for _, item := range l {
		if item.Type == sourceType && item.SourceID == sourceID {
			return true
		}
	}
	return false
}
