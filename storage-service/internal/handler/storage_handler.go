package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"entra-api/shared/middleware"
	"entra-api/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

type StorageHandler struct {
	minioClient   *minio.Client
	bucketName    string
	minioEndpoint string
	publicBaseURL string
}

func NewStorageHandler(minioClient *minio.Client, bucketName, minioEndpoint, publicBaseURL string) *StorageHandler {
	if publicBaseURL == "" {
		publicBaseURL = os.Getenv("STORAGE_PUBLIC_URL")
		if publicBaseURL == "" {
			publicBaseURL = os.Getenv("MINIO_PUBLIC_URL")
		}
	}
	if publicBaseURL == "" {
		publicBaseURL = fmt.Sprintf("http://%s", minioEndpoint)
	}
	publicBaseURL = strings.TrimSuffix(publicBaseURL, "/")

	return &StorageHandler{
		minioClient:   minioClient,
		bucketName:    bucketName,
		minioEndpoint: minioEndpoint,
		publicBaseURL: publicBaseURL,
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

	// Validate file size (5MB max)
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

	// Validate magic bytes MIME header (BUG-BE-12)
	header := make([]byte, 512)
	n, err := openedFile.Read(header)
	if err != nil && err != io.EOF {
		response.Error(c, http.StatusInternalServerError, "Failed to read file content")
		return
	}
	if _, err := openedFile.Seek(0, io.SeekStart); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to reset file read pointer")
		return
	}

	detectedMIME := http.DetectContentType(header[:n])
	if detectedMIME != "image/jpeg" && detectedMIME != "image/png" {
		response.Error(c, http.StatusBadRequest, "Invalid file content: only valid JPEG and PNG images are allowed")
		return
	}

	// Safe type assertion for user/organizer ID (BUG-BE-10)
	userID, exists := c.Get(middleware.AuthUserIDKey)
	organizerID, ok := userID.(string)
	if !exists || !ok || organizerID == "" {
		response.Unauthorized(c, "unauthorized")
		return
	}

	// Generate unique filename prefixed by organizer ID
	newFilename := fmt.Sprintf("%s/%s%s", organizerID, uuid.New().String(), ext)

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

	// Construct public URL using publicBaseURL (BUG-BE-11)
	publicURL := fmt.Sprintf("%s/%s/%s", h.publicBaseURL, h.bucketName, newFilename)

	response.Success(c, http.StatusOK, "File uploaded successfully", gin.H{
		"url": publicURL,
	})
}

func (h *StorageHandler) ListFiles(c *gin.Context) {
	ctx := context.Background()
	var images []gin.H

	userID, exists := c.Get(middleware.AuthUserIDKey)
	organizerID, ok := userID.(string)
	if !exists || !ok || organizerID == "" {
		response.Unauthorized(c, "unauthorized")
		return
	}

	prefix := organizerID + "/"

	for object := range h.minioClient.ListObjects(ctx, h.bucketName, minio.ListObjectsOptions{Prefix: prefix}) {
		if object.Err != nil {
			fmt.Println("Error listing object:", object.Err)
			continue
		}

		publicURL := fmt.Sprintf("%s/%s/%s", h.publicBaseURL, h.bucketName, object.Key)
		sizeKB := object.Size / 1024
		sizeStr := fmt.Sprintf("%d KB", sizeKB)
		if sizeKB > 1024 {
			sizeStr = fmt.Sprintf("%.1f MB", float64(sizeKB)/1024.0)
		}

		name := strings.TrimPrefix(object.Key, prefix)

		images = append(images, gin.H{
			"id":   object.Key,
			"url":  publicURL,
			"name": name,
			"size": sizeStr,
		})
	}

	// Ensure we return an empty array instead of null if no images
	if images == nil {
		images = []gin.H{}
	}

	response.Success(c, http.StatusOK, "Media retrieved", images)
}
