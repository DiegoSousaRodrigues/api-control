package service

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestValidateAndPrepareImageRejectsUnsupportedMagicBytes(t *testing.T) {
	_, _, err := validateAndPrepareImage(&readSeekCloser{Reader: bytes.NewReader([]byte("not an image"))})

	if !errors.Is(err, ErrUploadUnsupportedType) {
		t.Fatalf("err = %v, want ErrUploadUnsupportedType", err)
	}
}

func TestValidateAndPrepareImageDetectsPNGAndPreservesReader(t *testing.T) {
	imageBytes := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, []byte("payload")...)

	reader, contentType, err := validateAndPrepareImage(&readSeekCloser{Reader: bytes.NewReader(imageBytes)})
	if err != nil {
		t.Fatalf("validateAndPrepareImage returned error: %v", err)
	}

	if contentType != "image/png" {
		t.Fatalf("contentType = %q, want image/png", contentType)
	}

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}

	if !bytes.Equal(got, imageBytes) {
		t.Fatalf("reader content was not preserved")
	}
}

func TestGenerateBlobFilenameIgnoresUserFilenameShape(t *testing.T) {
	filename, err := generateBlobFilename("image/jpeg")
	if err != nil {
		t.Fatalf("generateBlobFilename returned error: %v", err)
	}

	if !strings.HasPrefix(filename, "sku-image-") || !strings.HasSuffix(filename, ".jpg") {
		t.Fatalf("filename = %q, want generated sku-image with .jpg extension", filename)
	}

	if strings.Contains(filename, "..") || strings.ContainsAny(filename, `/\`) {
		t.Fatalf("filename = %q, want no path traversal separators", filename)
	}
}

func TestValidateUploadSizeRejectsOversizedFile(t *testing.T) {
	err := validateUploadSize(&multipart.FileHeader{Size: MaxUploadFileBytes + 1})

	if !errors.Is(err, ErrUploadFileTooLarge) {
		t.Fatalf("err = %v, want ErrUploadFileTooLarge", err)
	}
}

func TestUploadToVercelBlobUsesTimeoutContextAndDoesNotLeakTokenInErrors(t *testing.T) {
	const secretToken = "vercel_blob_secret_token"

	originalBaseURL := blobAPIBaseURL
	originalClient := blobHTTPClient
	originalToken := os.Getenv("BLOB_READ_WRITE_TOKEN")
	t.Cleanup(func() {
		blobAPIBaseURL = originalBaseURL
		blobHTTPClient = originalClient
		_ = os.Setenv("BLOB_READ_WRITE_TOKEN", originalToken)
	})

	blobAPIBaseURL = "https://blob.test"
	blobHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+secretToken {
			t.Fatalf("Authorization header = %q", got)
		}

		if _, ok := r.Context().Deadline(); !ok {
			t.Fatalf("request context has no deadline")
		}

		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader("storage rejected")),
			Header:     make(http.Header),
		}, nil
	})}
	_ = os.Setenv("BLOB_READ_WRITE_TOKEN", secretToken)

	_, err := UploadToVercelBlob(strings.NewReader("image-bytes"), "sku-image-test.png", "image/png")
	if err == nil {
		t.Fatalf("UploadToVercelBlob returned nil error")
	}

	if strings.Contains(err.Error(), secretToken) {
		t.Fatalf("error leaked blob token: %v", err)
	}
}

type readSeekCloser struct {
	*bytes.Reader
}

func (r *readSeekCloser) Close() error {
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
