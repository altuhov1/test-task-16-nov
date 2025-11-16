package services

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"status-links/internal/models"
	"status-links/internal/storage"
	"sync"
	"time"

	"github.com/jung-kurt/gofpdf"
)

var (
	ErrTooBigIndex = errors.New("too big index")
)

type LinksService struct {
	temp     storage.TempStorage
	reliable storage.ReliableStorage
	client   *http.Client
	wg       sync.WaitGroup
}

func NewLinksService(temp storage.TempStorage, reliable storage.ReliableStorage) *LinksService {
	service := &LinksService{
		temp:     temp,
		reliable: reliable,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	service.uploadAllToFastMem()
	return service
}

func (l *LinksService) UploadAllUnfinishedWork() *models.AllUnfinishedWork {
	pendingLinks, err := l.reliable.GetPendingLinksData()
	if err != nil {
		slog.Error("Error getting pending links", "error", err)
		return &models.AllUnfinishedWork{
			Pdfs: []models.ListOfProcessedLinks{
				{
					Description: fmt.Sprintf("Error getting pending links: %v", err),
				},
			},
		}
	}

	pendingNums, err := l.reliable.GetPendingNumsData()
	if err != nil {
		slog.Error("Error getting pending nums", "error", err)
		return &models.AllUnfinishedWork{
			Pdfs: []models.ListOfProcessedLinks{
				{
					Description: fmt.Sprintf("Error getting pending nums: %v", err),
				},
			},
		}
	}

	result := &models.AllUnfinishedWork{}

	if len(pendingLinks) == 0 && len(pendingNums) == 0 {
		return &models.AllUnfinishedWork{
			Pdfs: []models.ListOfProcessedLinks{
				{
					Description: "No unfinished work found",
				},
			},
		}
	}

	for _, linkSet := range pendingLinks {
		processed := l.processLinks(linkSet)
		if processed != nil {
			result.Links = append(result.Links, *processed)
		}
	}

	for _, numSet := range pendingNums {
		pdfResult := l.generatePDF(numSet)
		if pdfResult != nil {
			result.Pdfs = append(result.Pdfs, *pdfResult)
		}
	}

	slog.Info("UploadAllUnfinishedWork: returning processed work",
		"links", len(result.Links),
		"pdfs", len(result.Pdfs))

	return result
}
func (l *LinksService) uploadAllToFastMem() *models.ProcessedLinks {
	allData, err := l.reliable.ReadAllFile()
	if err != nil {
		return &models.ProcessedLinks{
			Answer:  make(models.LinksAnswer),
			ListNum: -1,
		}
	}

	l.temp.UploadAllData(allData)

	return &models.ProcessedLinks{
		Answer:  make(models.LinksAnswer),
		ListNum: len(*allData),
	}
}

func (l *LinksService) AddLinkSet(set models.SetLinksGet) *models.ProcessedLinks {
	hash, err := l.reliable.AddLinksProcessList(&set)
	if err != nil {
		slog.Error("error in AddLinksProcessList", "error", err)
	}

	answer := make(models.LinksAnswer)

	for _, url := range set.Links {
		status := l.checkLinkStatus(url)
		answer[url] = status
	}

	listNum := l.temp.UploadNewData(&models.ProcessedLinks{
		Answer: answer,
	})

	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		processed := &models.ProcessedLinks{
			Answer:  answer,
			ListNum: listNum,
		}
		if err := l.reliable.AddNewLinkPerm(processed); err != nil {
			slog.Error("failed to save processed links:", "error", err)
		}
	}()

	err = l.reliable.RemoveLinksProcessByHash(hash)
	if err != nil {
		slog.Error("error in RemoveLinksProcessByHash", "error", err)
	}
	return &models.ProcessedLinks{
		Answer:  answer,
		ListNum: listNum,
	}
}

func (l *LinksService) GiveLinkAnswer(list models.SetNumsOfLinksGet) (*models.ListOfProcessedLinks, error) {
	maxInt := l.temp.ReturnMaxIndex()
	for _, v := range list.NumsLinks {
		if v > maxInt {
			return nil, ErrTooBigIndex
		}
	}
	hash, err := l.reliable.AddNumProcessList(&list)
	if err != nil {
		slog.Error("error in AddNumProcessList", "error", err)

	}
	result := l.generatePDF(list)

	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		hash, err := l.reliable.AddNumProcessList(&list)
		if err != nil {
			slog.Error("failed to add to pending:", "error", err)
			return
		}
		l.reliable.RemoveNumsProcessByHash(hash)
	}()
	err = l.reliable.RemoveNumsProcessByHash(hash)
	if err != nil {
		slog.Error("error in RemoveNumsProcessByHash", "error", err)
	}
	return result, nil
}

func (l *LinksService) processLinks(set models.SetLinksGet) *models.ProcessedLinks {
	answer := make(models.LinksAnswer)

	for _, url := range set.Links {
		status := l.checkLinkStatus(url)
		answer[url] = status
	}

	listNum := l.temp.UploadNewData(&models.ProcessedLinks{
		Answer: answer,
	})

	return &models.ProcessedLinks{
		Answer:  answer,
		ListNum: listNum,
	}
}

func (l *LinksService) checkLinkStatus(url string) string {
	fullURL := url
	if !hasScheme(url) {
		fullURL = "https://" + url
	}

	resp, err := l.client.Head(fullURL)
	if err != nil {
		return "unavailable"
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return "available"
	}
	return "unavailable"
}

func (l *LinksService) generatePDF(set models.SetNumsOfLinksGet) *models.ListOfProcessedLinks {
	linksAnswers, err := l.temp.FindKeys(&set)
	if err != nil {
		return &models.ListOfProcessedLinks{
			Description: fmt.Sprintf("Error finding keys: %v", err),
		}
	}

	if len(*linksAnswers) == 0 {
		return &models.ListOfProcessedLinks{
			Description: "No data found for the provided numbers",
			PDF:         []byte{},
		}
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "Links Status Report")
	pdf.Ln(12)

	pdf.SetFont("Arial", "", 12)

	row := 1
	for _, linkAnswer := range *linksAnswers {
		for url, status := range linkAnswer {
			statusText := "Available"
			if status == "unavailable" {
				statusText = "Unavailable"
			}
			pdf.Cell(0, 10, fmt.Sprintf("%d. %s - %s", row, url, statusText))
			pdf.Ln(6)
			row++
		}
	}

	var buf bytes.Buffer
	err = pdf.Output(&buf)
	if err != nil {
		return &models.ListOfProcessedLinks{
			Description: fmt.Sprintf("Error generating PDF: %v", err),
			PDF:         []byte{},
		}
	}

	return &models.ListOfProcessedLinks{
		Description: "PDF report generated successfully",
		PDF:         buf.Bytes(),
	}
}

func (l *LinksService) WaitForCompletion() {
	l.wg.Wait()
}
func hasScheme(url string) bool {
	return len(url) > 7 && (url[0:7] == "http://" || url[0:8] == "https://")
}
