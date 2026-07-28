package handler

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"entra-api/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

type StorageHandler struct {
	minioClient   *minio.Client
	bucketName    string
	minioEndpoint string
}

func NewStorageHandler(minioClient *minio.Client, bucketName, minioEndpoint string) *StorageHandler {
	return &StorageHandler{
		minioClient:   minioClient,
		bucketName:    bucketName,
		minioEndpoint: minioEndpoint,
	}
}

func (h *StorageHandler) UploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "No file is received")
		return
	}

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		response.Error(c, http.StatusBadRequest, "Only JPG, JPEG, and PNG files are allowed")
		return
	}

	// Validate file size (e.g., 5MB max)
	if file.Size > 5*1024*1024 {
		response.Error(c, http.StatusBadRequest, "File size exceeds 5MB limit")
		return
	}

	openedFile, err := file.Open()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to open the file")
		return
	}
	defer openedFile.Close()

	// Generate unique filename
	newFilename := uuid.New().String() + ext

	ctx := context.Background()
	
	contentType := "application/octet-stream"
	if ext == ".jpg" || ext == ".jpeg" {
		contentType = "image/jpeg"
	} else if ext == ".png" {
		contentType = "image/png"
	}

	_, err = h.minioClient.PutObject(ctx, h.bucketName, newFilename, openedFile, file.Size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to upload file to storage")
		return
	}

	// Construct public URL
	publicURL := fmt.Sprintf("http://%s/%s/%s", h.minioEndpoint, h.bucketName, newFilename)

	response.Success(c, http.StatusOK, "File uploaded successfully", gin.H{
		"url": publicURL,
	})
}
