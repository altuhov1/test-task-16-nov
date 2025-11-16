package services

import "status-links/internal/models"

type LinkProcessor interface {
	UploadAllUnfinishedWork() *models.AllUnfinishedWork
	AddLinkSet(set models.SetLinksGet) *models.ProcessedLinks
	GiveLinkAnswer(list models.SetNumsOfLinksGet) (*models.ListOfProcessedLinks, error)
	WaitForCompletion()
}
