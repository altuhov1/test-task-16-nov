package storage

import "status-links/internal/models"

type TempStorage interface {
	UploadAllData(bs *[]models.ProcessedLinks)
	UploadNewData(bs *models.ProcessedLinks) int
	FindKeys(list *models.SetNumsOfLinksGet) (*[]models.LinksAnswer, error)
}
type ReliableStorage interface {
	ReadAllFile() (*[]models.ProcessedLinks, error)
	AddNewLinkAllFileTask(item *models.ProcessedLinks) error
	AddLinksProcessList(masLinks *models.SetLinksGet) (string, error)
	AddNumProcessList(masLinks *models.SetNumsOfLinksGet) (string, error)
	RemoveLinksProcessByHash(targetHash string) error
	RemoveNumsProcessByHash(targetHash string) error
	GetPendingLinksData() ([]models.SetLinksGet, error)
	GetPendingNumsData() ([]models.SetNumsOfLinksGet, error)
}
