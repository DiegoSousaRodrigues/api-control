package service

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/api-control/internal/dto"
	"github.com/api-control/internal/repository"
)

var SkuService ISkuService = &skuService{}

type ISkuService interface {
	List() (*[]dto.SkuDTO, error)
	Add(clientDTO dto.SkuDTO) (err error)
	ChangeStatus(id string, status string) error
	FindByID(id string) (*dto.SkuDTO, error)
	Update(id string, skuDto dto.SkuDTO) (err error)
}

type skuService struct{}

func (s *skuService) List() (*[]dto.SkuDTO, error) {
	listEntity, err := repository.SkuRepository.List()
	if err != nil {
		return nil, err
	}

	var listDTO []dto.SkuDTO

	for _, value := range *listEntity {
		listDTO = append(listDTO, dto.ParseSkuToDTO(value))
	}

	return &listDTO, nil
}

func (c *skuService) Add(skuDto dto.SkuDTO) (err error) {
	file, err := skuDto.File.Open()
	if err != nil {
		return err
	}
	defer file.Close()

	// --- INICIO DO DEBUG: SALVAR LOCALMENTE ---
	// Cria um arquivo local com prefixo "debug_"
	out, err := os.Create("debug_" + skuDto.File.Filename)
	if err != nil {
		fmt.Println("Erro ao criar arquivo de debug:", err)
	} else {
		// Copia o conteúdo do upload para esse arquivo local
		_, err = io.Copy(out, file)
		out.Close() // Fecha o arquivo local
		if err != nil {
			fmt.Println("Erro ao salvar arquivo de debug:", err)
		} else {
			fmt.Println("DEBUG: Arquivo salvo localmente como debug_" + skuDto.File.Filename)
		}
	}

	// CRUCIAL: Reseta o ponteiro do arquivo para o início (byte 0)
	// Se não fizer isso, o UploadToVercelBlob vai ler a partir do final (vazio)
	_, err = file.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("erro ao resetar ponteiro do arquivo: %w", err)
	}
	// --- FIM DO DEBUG ---

	blobResp, err := UploadToVercelBlob(file, skuDto.File.Filename, skuDto.File.Header.Get("Content-Type"))
	if err != nil {
		return err
	}

	fmt.Println("Imagem salva em:", blobResp.Url)

	entity, err := dto.ParseSkuRequestToEntity(skuDto)
	if err != nil {
		return err
	}

	err = repository.SkuRepository.Add(*entity)
	if err != nil {
		return err
	}

	return nil
}

func (c *skuService) ChangeStatus(id string, status string) error {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return err
	}

	statusBool, err := strconv.ParseBool(status)
	if err != nil {
		return err
	}

	err = repository.SkuRepository.ChangeStatus(int64(idInt), statusBool)
	if err != nil {
		return err
	}

	return nil
}

func (c *skuService) FindByID(id string) (*dto.SkuDTO, error) {
	entity, err := repository.SkuRepository.FindByID(id)
	if err != nil {
		return nil, err
	}

	dtoSku := dto.ParseSkuToDTO(*entity)
	return &dtoSku, nil
}

func (c *skuService) Update(id string, skuDto dto.SkuDTO) (err error) {
	entity, err := dto.ParseSkuRequestToEntity(skuDto)
	if err != nil {
		return err
	}

	intId, err := strconv.Atoi(id)
	if err != nil {
		return err
	}

	err = repository.SkuRepository.Update(int64(intId), *entity)
	if err != nil {
		return nil
	}

	return nil
}
