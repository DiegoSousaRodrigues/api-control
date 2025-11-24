package service

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

type BlobResponse struct {
	Url         string `json:"url"`
	DownloadUrl string `json:"downloadUrl"`
	Pathname    string `json:"pathname"`
	ContentType string `json:"contentType"`
}

// UploadToVercelBlob aceita um Reader (arquivo aberto) e o nome que você quer dar
func UploadToVercelBlob(file io.Reader, filename string, contentType string) (*BlobResponse, error) {
	// _ = godotenv.Load() // Se já carregou no main.go, não precisa carregar aqui toda vez

	token := os.Getenv("BLOB_READ_WRITE_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("BLOB_READ_WRITE_TOKEN não configurado")
	}

	// A URL da API REST da Vercel Blob é baseada no nome do arquivo
	apiURL := fmt.Sprintf("https://blob.vercel-storage.com/%s", filename)

	req, err := http.NewRequest("PUT", apiURL, file)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("x-api-version", "1")
	// "1" adiciona um sufixo aleatório para evitar sobrescrever arquivos (ex: foto-xh52.png)
	// Use "0" se quiser sobrescrever sempre o mesmo arquivo.
	req.Header.Set("x-random-suffix", "1")
	req.Header.Set("Content-Type", contentType)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("erro Vercel Blob (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var blobResp BlobResponse
	if err := json.NewDecoder(resp.Body).Decode(&blobResp); err != nil {
		return nil, err
	}

	return &blobResp, nil
}

func LoadUploadToVercelBlob(file *multipart.FileHeader) (*string, error) {
	var imageUrl *string

	if file != nil {
		_file, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer _file.Close()

		resp, err := UploadToVercelBlob(_file, file.Filename, file.Header.Get("Content-Type"))
		if err != nil {
			return nil, err
		}

		imageUrl = &resp.Url
	}

	return imageUrl, nil
}
