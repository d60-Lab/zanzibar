package dto

// CheckPermissionRequest represents a permission check request
type CheckPermissionRequest struct {
	UserID         string `json:"user_id" binding:"required"`
	DocumentID     string `json:"document_id" binding:"required"`
	PermissionType string `json:"permission_type" binding:"required,oneof=viewer editor owner"`
}

// CheckPermissionBatchRequest represents a batch permission check request
type CheckPermissionBatchRequest struct {
	UserID         string   `json:"user_id" binding:"required"`
	DocumentIDs    []string `json:"document_ids" binding:"required,min=1,max=100"`
	PermissionType string   `json:"permission_type" binding:"required,oneof=viewer editor owner"`
}

// GrantPermissionRequest represents a grant permission request
type GrantPermissionRequest struct {
	UserID         string `json:"user_id" binding:"required"`
	DocumentID     string `json:"document_id" binding:"required"`
	PermissionType string `json:"permission_type" binding:"required,oneof=viewer editor owner"`
}

// AddCustomerFollowerRequest represents an add customer follower request
type AddCustomerFollowerRequest struct {
	CustomerID string `json:"customer_id" binding:"required"`
	UserID     string `json:"user_id" binding:"required"`
}

// UpdateDepartmentManagerRequest represents an update department manager request
type UpdateDepartmentManagerRequest struct {
	DepartmentID string `json:"department_id" binding:"required"`
	ManagerID    string `json:"manager_id" binding:"required"`
}

// AddUserToDepartmentRequest represents an add user to department request
type AddUserToDepartmentRequest struct {
	UserID       string `json:"user_id" binding:"required"`
	DepartmentID string `json:"department_id" binding:"required"`
	Role         string `json:"role" binding:"required,oneof=member leader director"`
	IsPrimary    bool   `json:"is_primary"`
}

// GenerateTestDataRequest represents a test data generation request
type GenerateTestDataRequest struct {
	NumUsers          int `json:"num_users" binding:"min=1,max=100000"`
	NumDepartments    int `json:"num_departments" binding:"min=1,max=10000"`
	NumCustomers      int `json:"num_customers" binding:"min=1,max=1000000"`
	NumDocuments      int `json:"num_documents" binding:"min=1,max=5000000"`
	MaxDeptLevels     int `json:"max_dept_levels" binding:"min=1,max=10"`
	MaxDeptMembers    int `json:"max_dept_members" binding:"min=1,max=1000"`
	MaxCustomerFollowers int `json:"max_customer_followers" binding:"min=1,max=100"`
	BatchSize         int `json:"batch_size" binding:"min=1,max=10000"`
}

// BenchmarkRequest represents a benchmark execution request
type BenchmarkRequest struct {
	TestName      string `json:"test_name" binding:"required"`
	EngineType    string `json:"engine_type" binding:"required,oneof=mysql zanzibar both"`
	TestCategory  string `json:"test_category" binding:"required,oneof=read write scalability realworld"`
	Iterations    int    `json:"iterations" binding:"min=1,max=10000"`
	Concurrency   int    `json:"concurrency" binding:"min=1,max=1000"`
}

// BenchmarkResponse represents a benchmark execution response
type BenchmarkResponse struct {
	BenchmarkID int64  `json:"benchmark_id"`
	Status      string `json:"status"`
	Message     string `json:"message"`
}

// GetStorageComparisonResponse represents storage comparison response
type GetStorageComparisonResponse struct {
	MySQL    StorageStats `json:"mysql"`
	Zanzibar StorageStats `json:"zanzibar"`
}

// StorageStats represents storage statistics
type StorageStats struct {
	EngineType  string  `json:"engine_type"`
	TableName   string  `json:"table_name"`
	RowCount    int64   `json:"row_count"`
	DataSizeMB  float64 `json:"data_size_mb"`
	IndexSizeMB float64 `json:"index_size_mb"`
	TotalSizeMB float64 `json:"total_size_mb"`
}
