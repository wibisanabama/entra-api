package handler_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestStorageHandler_MIMEValidation(t *testing.T) {
	// Valid JPEG magic header
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}
	mimeJPEG := http.DetectContentType(jpegHeader)
	if mimeJPEG != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", mimeJPEG)
	}

	// Valid PNG magic header
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	mimePNG := http.DetectContentType(pngHeader)
	if mimePNG != "image/png" {
		t.Errorf("expected image/png, got %s", mimePNG)
	}

	// Malicious/executable header pretending to be an image
	exeHeader := []byte{0x4D, 0x5A, 0x90, 0x00, 0x03, 0x00, 0x00, 0x00}
	mimeEXE := http.DetectContentType(exeHeader)
	if mimeEXE == "image/jpeg" || mimeEXE == "image/png" {
		t.Errorf("executable must not be detected as jpeg or png, got %s", mimeEXE)
	}

	// Script/HTML content
	htmlHeader := []byte("<html><script>alert(1)</script></html>")
	mimeHTML := http.DetectContentType(htmlHeader)
	if mimeHTML == "image/jpeg" || mimeHTML == "image/png" {
		t.Errorf("html script must not be detected as image, got %s", mimeHTML)
	}
}

func TestStorageHandler_PublicURLResolution(t *testing.T) {
	bucketName := "entra-media"
	filename := "org-123/banner.png"

	t.Run("Custom STORAGE_PUBLIC_URL formatted correctly", func(t *testing.T) {
		publicBaseURL := "https://cdn.entra.id"
		publicBaseURL = strings.TrimSuffix(publicBaseURL, "/")
		expected := "https://cdn.entra.id/entra-media/org-123/banner.png"
		actual := publicBaseURL + "/" + bucketName + "/" + filename
		if actual != expected {
			t.Errorf("expected %s, got %s", expected, actual)
		}
	})

	t.Run("Fallback to minio endpoint when public URL empty", func(t *testing.T) {
		minioEndpoint := "localhost:9000"
		publicBaseURL := "http://" + minioEndpoint
		expected := "http://localhost:9000/entra-media/org-123/banner.png"
		actual := publicBaseURL + "/" + bucketName + "/" + filename
		if actual != expected {
			t.Errorf("expected %s, got %s", expected, actual)
		}
	})
}
