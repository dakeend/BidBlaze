package upload

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "golang.org/x/image/webp"
)

const (
	MaxFileSize         = 5 * 1024 * 1024
	defaultUploadDir    = "./uploads"
	defaultPublicPrefix = "/static"
)

var (
	errInvalidAuth  = errors.New("invalid auth token")
	errInvalidImage = errors.New("invalid image")
	errTooLarge     = errors.New("file too large")
	tokenPattern    = regexp.MustCompile(`^mock-token-(seller|user)-[0-9]{3,}$`)
)

type Result struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Size   int64  `json:"size"`
}

type Handler struct {
	service *Service
}

type Service struct {
	uploadDir    string
	publicPrefix string
	now          func() time.Time
}

type imageInfo struct {
	ext  string
	size image.Point
}

func NewHandlerFromEnv() *Handler {
	return NewHandler(NewService(ConfigFromEnv()))
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func NewService(uploadDir, publicPrefix string) *Service {
	if strings.TrimSpace(uploadDir) == "" {
		uploadDir = defaultUploadDir
	}
	return &Service{
		uploadDir:    uploadDir,
		publicPrefix: strings.TrimRight(publicPrefix, "/"),
		now:          time.Now,
	}
}

func ConfigFromEnv() (string, string) {
	return os.Getenv("UPLOAD_DIR"), os.Getenv("UPLOAD_PUBLIC_PREFIX")
}

func RegisterRoutes(router gin.IRoutes, handler *Handler) {
	router.POST("/api/uploads", handler.ServeUpload)
}

func StaticDirFromEnv() string {
	dir := strings.TrimSpace(os.Getenv("UPLOAD_DIR"))
	if dir == "" {
		return defaultUploadDir
	}
	return dir
}

func (h *Handler) ServeUpload(c *gin.Context) {
	if err := requireAuth(c.GetHeader("Authorization")); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 1002, "msg": "unauthorized", "data": nil})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxFileSize+1024*1024)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "msg": "file is required", "data": nil})
		return
	}

	result, err := h.service.SaveWithPublicPrefix(fileHeader, h.publicPrefix(c))
	if err != nil {
		status := http.StatusBadRequest
		msg := "invalid upload file"
		if errors.Is(err, errTooLarge) {
			msg = "file size must be <= 5MB"
		}
		c.JSON(status, gin.H{"code": 1001, "msg": msg, "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": result})
}

func (s *Service) Save(fileHeader *multipart.FileHeader) (Result, error) {
	return s.SaveWithPublicPrefix(fileHeader, s.publicPrefix)
}

func (s *Service) SaveWithPublicPrefix(fileHeader *multipart.FileHeader, publicPrefix string) (Result, error) {
	if fileHeader == nil {
		return Result{}, errInvalidImage
	}
	if fileHeader.Size <= 0 || fileHeader.Size > MaxFileSize {
		return Result{}, errTooLarge
	}

	file, err := fileHeader.Open()
	if err != nil {
		return Result{}, errInvalidImage
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, MaxFileSize+1))
	if err != nil {
		return Result{}, errInvalidImage
	}
	if int64(len(data)) > MaxFileSize {
		return Result{}, errTooLarge
	}

	info, err := inspectImage(data)
	if err != nil {
		return Result{}, err
	}

	now := s.now()
	parts := []string{
		fmt.Sprintf("%04d", now.Year()),
		fmt.Sprintf("%02d", int(now.Month())),
		fmt.Sprintf("%02d", now.Day()),
	}

	filename, err := uniqueFilename(data, info.ext)
	if err != nil {
		return Result{}, errInvalidImage
	}

	targetDir, err := safeDateDir(s.uploadDir, parts)
	if err != nil {
		return Result{}, errInvalidImage
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return Result{}, err
	}

	written := false
	for attempt := 0; attempt < 3; attempt++ {
		targetPath := filepath.Join(targetDir, filename)
		if err := writeNewFile(targetPath, data); err != nil {
			if errors.Is(err, os.ErrExist) {
				filename, err = uniqueFilename(data, info.ext)
				if err != nil {
					return Result{}, errInvalidImage
				}
				continue
			}
			return Result{}, err
		}
		written = true
		break
	}
	if !written {
		return Result{}, errInvalidImage
	}

	urlParts := append(parts, filename)
	return Result{
		URL:    joinPublicURL(publicPrefix, urlParts),
		Width:  info.size.X,
		Height: info.size.Y,
		Size:   int64(len(data)),
	}, nil
}

func (h *Handler) publicPrefix(c *gin.Context) string {
	if h.service.publicPrefix != "" {
		return h.service.publicPrefix
	}

	scheme := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if scheme == "" {
		scheme = "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
	}

	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(c.Request.Host)
	}
	if host == "" {
		return defaultPublicPrefix
	}
	return scheme + "://" + host + defaultPublicPrefix
}

func requireAuth(header string) error {
	const bearer = "Bearer "
	if !strings.HasPrefix(header, bearer) {
		return errInvalidAuth
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, bearer))
	// TODO(prod): replace with JWT or safer upload auth.
	if !tokenPattern.MatchString(token) {
		return errInvalidAuth
	}
	return nil
}

func inspectImage(data []byte) (imageInfo, error) {
	_, ext, ok := detectMagic(data)
	if !ok {
		return imageInfo{}, errInvalidImage
	}

	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return imageInfo{}, errInvalidImage
	}

	return imageInfo{
		ext:  ext,
		size: image.Point{X: config.Width, Y: config.Height},
	}, nil
}

func detectMagic(data []byte) (string, string, bool) {
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "image/jpeg", ".jpg", true
	}
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return "image/png", ".png", true
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp", ".webp", true
	}
	return "", "", false
}

func uniqueFilename(data []byte, ext string) (string, error) {
	hash := sha256.Sum256(data)
	random := make([]byte, 4)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s%s", hex.EncodeToString(hash[:])[:16], hex.EncodeToString(random), ext), nil
}

func writeNewFile(targetPath string, data []byte) error {
	file, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	return err
}

func safeDateDir(root string, parts []string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	target := absRoot
	for _, part := range parts {
		if part == "" || strings.Contains(part, "..") || strings.ContainsAny(part, `/\`) {
			return "", errInvalidImage
		}
		target = filepath.Join(target, part)
	}

	rel, err := filepath.Rel(absRoot, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errInvalidImage
	}
	return target, nil
}

func joinPublicURL(prefix string, parts []string) string {
	cleanPrefix := strings.TrimRight(prefix, "/")
	if cleanPrefix == "" {
		cleanPrefix = defaultPublicPrefix
	}
	return cleanPrefix + "/" + path.Join(parts...)
}
