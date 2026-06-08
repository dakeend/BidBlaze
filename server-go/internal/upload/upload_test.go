package upload

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestServeUploadAcceptsPNG(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tempDir := t.TempDir()
	service := NewService(tempDir, "/static")
	service.now = func() time.Time {
		return time.Date(2026, 6, 3, 12, 0, 0, 0, time.Local)
	}

	router := gin.New()
	RegisterRoutes(router, NewHandler(service))

	request := multipartUploadRequest(t, "/api/uploads", "product.png", newPNG(t))
	request.Header.Set("Authorization", "Bearer mock-token-seller-001")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var body struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data Result `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != 0 || body.Msg != "ok" {
		t.Fatalf("unexpected response: %+v", body)
	}
	if body.Data.Width != 1 || body.Data.Height != 1 || body.Data.Size <= 0 {
		t.Fatalf("unexpected image metadata: %+v", body.Data)
	}
	if !strings.HasPrefix(body.Data.URL, "/static/2026/06/03/") {
		t.Fatalf("unexpected public url: %s", body.Data.URL)
	}
	if strings.Contains(body.Data.URL, tempDir) || strings.Contains(body.Data.URL, `:\`) {
		t.Fatalf("public url leaked local path: %s", body.Data.URL)
	}
}

func TestServeUploadRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router, NewHandler(NewService(t.TempDir(), "/static")))

	request := multipartUploadRequest(t, "/api/uploads", "product.png", newPNG(t))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
	assertResponseCode(t, recorder.Body.Bytes(), 1002)
}

func TestServeUploadRejectsInvalidMagic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router, NewHandler(NewService(t.TempDir(), "/static")))

	request := multipartUploadRequest(t, "/api/uploads", "product.png", []byte("not a real png"))
	request.Header.Set("Authorization", "Bearer mock-token-seller-001")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	assertResponseCode(t, recorder.Body.Bytes(), 1001)
}

func TestServeUploadRejectsOversizedFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router, NewHandler(NewService(t.TempDir(), "/static")))

	request := multipartUploadRequest(t, "/api/uploads", "large.png", bytes.Repeat([]byte{'x'}, MaxFileSize+1))
	request.Header.Set("Authorization", "Bearer mock-token-seller-001")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	assertResponseCode(t, recorder.Body.Bytes(), 1001)
}

func multipartUploadRequest(t *testing.T, target string, filename string, payload []byte) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(payload); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, target, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func newPNG(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 12, G: 34, B: 56, A: 255})
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buffer.Bytes()
}

func assertResponseCode(t *testing.T, payload []byte, expected int) {
	t.Helper()

	var body struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != expected {
		t.Fatalf("expected code %d, got %d", expected, body.Code)
	}
}
