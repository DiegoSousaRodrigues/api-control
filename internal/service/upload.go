package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	MaxUploadFileBytes    int64 = 5 * 1024 * 1024
	MaxUploadRequestBytes int64 = MaxUploadFileBytes + 1*1024*1024
	MaxBlobResponseBytes  int64 = 32 * 1024
	blobUploadTimeout           = 15 * time.Second
)

var (
	ErrUploadFileTooLarge    = errors.New("imagem excede o tamanho maximo permitido")
	ErrUploadUnsupportedType = errors.New("tipo de imagem nao permitido")
	ErrUploadEmptyFile       = errors.New("imagem vazia")

	blobAPIBaseURL = "https://blob.vercel-storage.com"
	blobHTTPClient = &http.Client{Timeout: blobUploadTimeout}
)

type BlobResponse struct {
	Url         string `json:"url"`
	DownloadUrl string `json:"downloadUrl"`
	Pathname    string `json:"pathname"`
	ContentType string `json:"contentType"`
}

func UploadToVercelBlob(file io.Reader, filename string, contentType string) (*BlobResponse, error) {
	token := os.Getenv("BLOB_READ_WRITE_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("servico de upload nao configurado")
	}

	apiURL := fmt.Sprintf("%s/%s", strings.TrimRight(blobAPIBaseURL, "/"), filename)

	ctx, cancel := context.WithTimeout(context.Background(), blobUploadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiURL, file)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("x-api-version", "1")
	req.Header.Set("x-random-suffix", "1")
	req.Header.Set("Content-Type", contentType)

	resp, err := blobHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, MaxBlobResponseBytes))
		return nil, fmt.Errorf("falha ao enviar imagem para o storage (%d)", resp.StatusCode)
	}

	var blobResp BlobResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, MaxBlobResponseBytes)).Decode(&blobResp); err != nil {
		return nil, err
	}

	return &blobResp, nil
}

func LoadUploadToVercelBlob(file *multipart.FileHeader) (*string, error) {
	var imageUrl *string

	if file != nil {
		if err := validateUploadSize(file); err != nil {
			return nil, err
		}

		openedFile, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer openedFile.Close()

		imageReader, contentType, err := validateAndPrepareImage(openedFile)
		if err != nil {
			return nil, err
		}

		filename, err := generateBlobFilename(contentType)
		if err != nil {
			return nil, err
		}

		resp, err := UploadToVercelBlob(imageReader, filename, contentType)
		if err != nil {
			return nil, err
		}

		imageUrl = &resp.Url
	}

	return imageUrl, nil
}

func validateUploadSize(file *multipart.FileHeader) error {
	if file.Size <= 0 {
		return ErrUploadEmptyFile
	}

	if file.Size > MaxUploadFileBytes {
		return ErrUploadFileTooLarge
	}

	return nil
}

func validateAndPrepareImage(file multipart.File) (io.Reader, string, error) {
	head := make([]byte, 512)
	n, err := file.Read(head)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, "", err
	}

	if n == 0 {
		return nil, "", ErrUploadEmptyFile
	}

	head = head[:n]
	contentType := detectImageContentType(head)
	if contentType == "" {
		return nil, "", ErrUploadUnsupportedType
	}

	return io.MultiReader(bytes.NewReader(head), file), contentType, nil
}

func detectImageContentType(head []byte) string {
	detected := http.DetectContentType(head)
	switch detected {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return detected
	}

	if len(head) >= 12 && bytes.Equal(head[0:4], []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WEBP")) {
		return "image/webp"
	}

	return ""
}

func generateBlobFilename(contentType string) (string, error) {
	extensionByType := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/gif":  ".gif",
		"image/webp": ".webp",
	}

	extension, ok := extensionByType[contentType]
	if !ok {
		return "", ErrUploadUnsupportedType
	}

	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	return fmt.Sprintf("sku-image-%d-%s%s", time.Now().UTC().UnixNano(), hex.EncodeToString(randomBytes), extension), nil
}
