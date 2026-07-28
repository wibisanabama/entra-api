package handler

import (
	"entra-api/shared/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, storageHandler *StorageHandler, jwtSecret string) {
	v1 := r.Group("/api/v1")
	{
		storage := v1.Group("/storage")
		storage.Use(middleware.JWTAuth(jwtSecret))
		{
			storage.POST("/upload", storageHandler.UploadFile)
		}
	}
}
