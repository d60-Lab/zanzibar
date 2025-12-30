package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/d60-Lab/gin-template/internal/dto"
	"github.com/d60-Lab/gin-template/internal/repository"
)

// PermissionHandler handles permission-related HTTP requests
type PermissionHandler struct {
	mysqlRepo    *repository.MySQLPermissionRepository
	zanzibarRepo *repository.ZanzibarPermissionRepository
}

// NewPermissionHandler creates a new permission handler
func NewPermissionHandler(
	mysqlRepo *repository.MySQLPermissionRepository,
	zanzibarRepo *repository.ZanzibarPermissionRepository,
) *PermissionHandler {
	return &PermissionHandler{
		mysqlRepo:    mysqlRepo,
		zanzibarRepo: zanzibarRepo,
	}
}

// CheckPermissionMySQL checks permission using MySQL engine
// @Summary Check permission (MySQL)
// @Tags MySQL Permissions
// @Accept json
// @Produce json
// @Param request body dto.CheckPermissionRequest true "Permission check request"
// @Success 200 {object} model.PermissionCheckResult
// @Router /api/v1/permissions/mysql/check [post]
func (h *PermissionHandler) CheckPermissionMySQL(c *gin.Context) {
	var req dto.CheckPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.mysqlRepo.CheckPermission(c.Request.Context(), req.UserID, req.DocumentID, req.PermissionType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// CheckPermissionZanzibar checks permission using Zanzibar engine
// @Summary Check permission (Zanzibar)
// @Tags Zanzibar Permissions
// @Accept json
// @Produce json
// @Param request body dto.CheckPermissionRequest true "Permission check request"
// @Success 200 {object} model.PermissionCheckResult
// @Router /api/v1/permissions/zanzibar/check [post]
func (h *PermissionHandler) CheckPermissionZanzibar(c *gin.Context) {
	var req dto.CheckPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.zanzibarRepo.CheckPermission(c.Request.Context(), req.UserID, req.DocumentID, req.PermissionType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// CheckPermissionBoth checks permission using both engines for comparison
// @Summary Check permission (Both engines)
// @Tags Comparison
// @Accept json
// @Produce json
// @Param request body dto.CheckPermissionRequest true "Permission check request"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/permissions/both/check [post]
func (h *PermissionHandler) CheckPermissionBoth(c *gin.Context) {
	var req dto.CheckPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mysqlResult, err := h.mysqlRepo.CheckPermission(c.Request.Context(), req.UserID, req.DocumentID, req.PermissionType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "MySQL error: " + err.Error()})
		return
	}

	zanzibarResult, err := h.zanzibarRepo.CheckPermission(c.Request.Context(), req.UserID, req.DocumentID, req.PermissionType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Zanzibar error: " + err.Error()})
		return
	}

	// Verify both return the same result
	consistencyCheck := "✓"
	if mysqlResult.HasPermission != zanzibarResult.HasPermission {
		consistencyCheck = "✗ INCONSISTENT!"
	}

	c.JSON(http.StatusOK, gin.H{
		"mysql":          mysqlResult,
		"zanzibar":       zanzibarResult,
		"consistency":    consistencyCheck,
		"duration_diff":  zanzibarResult.DurationMs - mysqlResult.DurationMs,
	})
}

// GetUserDocumentsMySQL gets user's documents using MySQL engine
// @Summary Get user documents (MySQL)
// @Tags MySQL Permissions
// @Produce json
// @Param user_id path string true "User ID"
// @Param permission_type query string false "Permission type" Enums(viewer, editor, owner) default(viewer)
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} model.UserDocumentList
// @Router /api/v1/permissions/mysql/users/:user_id/documents [get]
func (h *PermissionHandler) GetUserDocumentsMySQL(c *gin.Context) {
	userID := c.Param("user_id")
	permissionType := c.DefaultQuery("permission_type", "viewer")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := h.mysqlRepo.GetUserDocuments(c.Request.Context(), userID, permissionType, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetUserDocumentsZanzibar gets user's documents using Zanzibar engine
// @Summary Get user documents (Zanzibar)
// @Tags Zanzibar Permissions
// @Produce json
// @Param user_id path string true "User ID"
// @Param permission_type query string false "Permission type" Enums(viewer, editor, owner) default(viewer)
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} model.UserDocumentList
// @Router /api/v1/permissions/zanzibar/users/:user_id/documents [get]
func (h *PermissionHandler) GetUserDocumentsZanzibar(c *gin.Context) {
	userID := c.Param("user_id")
	permissionType := c.DefaultQuery("permission_type", "viewer")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := h.zanzibarRepo.GetUserDocuments(c.Request.Context(), userID, permissionType, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GrantPermissionMySQL grants permission using MySQL engine
// @Summary Grant permission (MySQL)
// @Tags MySQL Permissions
// @Accept json
// @Produce json
// @Param request body dto.GrantPermissionRequest true "Grant permission request"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/permissions/mysql/grant [post]
func (h *PermissionHandler) GrantPermissionMySQL(c *gin.Context) {
	var req dto.GrantPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.mysqlRepo.GrantDirectPermission(c.Request.Context(), req.UserID, req.DocumentID, req.PermissionType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Permission granted successfully"})
}

// GrantPermissionZanzibar grants permission using Zanzibar engine
// @Summary Grant permission (Zanzibar)
// @Tags Zanzibar Permissions
// @Accept json
// @Produce json
// @Param request body dto.GrantPermissionRequest true "Grant permission request"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/permissions/zanzibar/grant [post]
func (h *PermissionHandler) GrantPermissionZanzibar(c *gin.Context) {
	var req dto.GrantPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.zanzibarRepo.GrantDirectPermission(c.Request.Context(), req.UserID, req.DocumentID, req.PermissionType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Permission granted successfully"})
}

// UpdateDepartmentManagerMySQL updates department manager (MySQL - EXPENSIVE!)
// @Summary Update department manager (MySQL - triggers full rebuild)
// @Tags MySQL Permissions
// @Accept json
// @Produce json
// @Param request body dto.UpdateDepartmentManagerRequest true "Update manager request"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/permissions/mysql/department/manager [post]
func (h *PermissionHandler) UpdateDepartmentManagerMySQL(c *gin.Context) {
	var req dto.UpdateDepartmentManagerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Copy values for goroutine - don't use request context
	departmentID := req.DepartmentID

	c.JSON(http.StatusOK, gin.H{
		"message": "Department manager update initiated (this will take a while...)",
		"warning": "This operation will rebuild millions of permission rows",
	})

	// Run in background with background context
	go func() {
		ctx := context.Background()
		if err := h.mysqlRepo.RebuildDepartmentPermissions(ctx, departmentID); err != nil {
			// Log error - in production, use proper logging
			fmt.Printf("Error rebuilding department permissions: %v\n", err)
		}
	}()
}

// UpdateDepartmentManagerZanzibar updates department manager (Zanzibar - FAST!)
// @Summary Update department manager (Zanzibar - single tuple update)
// @Tags Zanzibar Permissions
// @Accept json
// @Produce json
// @Param request body dto.UpdateDepartmentManagerRequest true "Update manager request"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/permissions/zanzibar/department/manager [post]
func (h *PermissionHandler) UpdateDepartmentManagerZanzibar(c *gin.Context) {
	var req dto.UpdateDepartmentManagerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.zanzibarRepo.UpdateDepartmentManager(c.Request.Context(), req.DepartmentID, req.ManagerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Department manager updated successfully",
		"note":    "Single tuple update - instant生效!",
	})
}

// GetStorageComparison returns storage statistics comparison
// @Summary Get storage comparison
// @Tags Comparison
// @Produce json
// @Success 200 {object} dto.GetStorageComparisonResponse
// @Router /api/v1/comparison/storage [get]
func (h *PermissionHandler) GetStorageComparison(c *gin.Context) {
	mysqlStats, err := h.mysqlRepo.GetStorageStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "MySQL stats error: " + err.Error()})
		return
	}

	zanzibarStats, err := h.zanzibarRepo.GetStorageStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Zanzibar stats error: " + err.Error()})
		return
	}

	reductionPct := 0.0
	if mysqlStats.RowCount > 0 {
		reductionPct = float64(mysqlStats.RowCount-zanzibarStats.RowCount) / float64(mysqlStats.RowCount) * 100
	}

	c.JSON(http.StatusOK, dto.GetStorageComparisonResponse{
		MySQL: dto.StorageStats{
			EngineType:  mysqlStats.EngineType,
			TableName:   mysqlStats.TableName,
			RowCount:    mysqlStats.RowCount,
			DataSizeMB:  mysqlStats.DataSizeMB,
			IndexSizeMB: mysqlStats.IndexSizeMB,
			TotalSizeMB: mysqlStats.TotalSizeMB,
		},
		Zanzibar: dto.StorageStats{
			EngineType:  zanzibarStats.EngineType,
			TableName:   zanzibarStats.TableName,
			RowCount:    zanzibarStats.RowCount,
			DataSizeMB:  zanzibarStats.DataSizeMB,
			IndexSizeMB: zanzibarStats.IndexSizeMB,
			TotalSizeMB: zanzibarStats.TotalSizeMB,
		},
		ReductionPct: reductionPct,
	})
}

// GetPermissionStatsMySQL returns permission statistics for MySQL
// @Summary Get permission stats (MySQL)
// @Tags MySQL Permissions
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Router /api/v1/permissions/mysql/stats [get]
func (h *PermissionHandler) GetPermissionStatsMySQL(c *gin.Context) {
	stats, err := h.mysqlRepo.GetPermissionStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"engine": "MySQL",
		"stats":  stats,
	})
}

// GetTupleStatsZanzibar returns tuple statistics for Zanzibar
// @Summary Get tuple stats (Zanzibar)
// @Tags Zanzibar Permissions
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Router /api/v1/permissions/zanzibar/stats [get]
func (h *PermissionHandler) GetTupleStatsZanzibar(c *gin.Context) {
	stats, err := h.zanzibarRepo.GetTupleStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"engine": "Zanzibar",
		"stats":  stats,
	})
}

// ClearZanzibarCache clears the Zanzibar permission cache
// @Summary Clear Zanzibar cache
// @Tags Zanzibar Permissions
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/permissions/zanzibar/cache/clear [post]
func (h *PermissionHandler) ClearZanzibarCache(c *gin.Context) {
	// Cache removed - no longer needed
	c.JSON(http.StatusOK, gin.H{"message": "Cache has been removed from the implementation"})
}
