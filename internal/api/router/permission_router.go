package router

import (
	"github.com/gin-gonic/gin"

	"github.com/d60-Lab/gin-template/internal/api/handler"
)

// SetupPermissionRoutes sets up all permission-related routes
func SetupPermissionRoutes(r *gin.Engine, permissionHandler *handler.PermissionHandler) {
	// API v1 group
	v1 := r.Group("/api/v1")
	{
		// MySQL Permission Routes
		mysql := v1.Group("/permissions/mysql")
		{
			mysql.POST("/check", permissionHandler.CheckPermissionMySQL)
			mysql.GET("/users/:user_id/documents", permissionHandler.GetUserDocumentsMySQL)
			mysql.POST("/grant", permissionHandler.GrantPermissionMySQL)
			mysql.POST("/department/manager", permissionHandler.UpdateDepartmentManagerMySQL)
			mysql.GET("/stats", permissionHandler.GetPermissionStatsMySQL)
		}

		// Zanzibar Permission Routes
		zanzibar := v1.Group("/permissions/zanzibar")
		{
			zanzibar.POST("/check", permissionHandler.CheckPermissionZanzibar)
			zanzibar.GET("/users/:user_id/documents", permissionHandler.GetUserDocumentsZanzibar)
			zanzibar.POST("/grant", permissionHandler.GrantPermissionZanzibar)
			zanzibar.POST("/department/manager", permissionHandler.UpdateDepartmentManagerZanzibar)
			zanzibar.GET("/stats", permissionHandler.GetTupleStatsZanzibar)
			zanzibar.POST("/cache/clear", permissionHandler.ClearZanzibarCache)
		}

		// Comparison Routes
		comparison := v1.Group("/comparison")
		{
			comparison.GET("/storage", permissionHandler.GetStorageComparison)
		}

		// Both engines comparison
		v1.POST("/permissions/both/check", permissionHandler.CheckPermissionBoth)
	}
}
