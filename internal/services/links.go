package services

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"status-links/internal/models"
	"status-links/internal/storage"
	"sync"
	"time"

	"github.com/jung-kurt/gofpdf"
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

// В services/link_service.go
func (l *LinksService) UploadAllUnfinishedWork() *models.AllUnfinishedWork {
	// Получаем незавершенные задачи из надежного хранилища
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

	slog.Info("UploadAllUnfinishedWork: found pending tasks",
		"pending_links", len(pendingLinks),
		"pending_nums", len(pendingNums))

	result := &models.AllUnfinishedWork{}

	// Если нет незавершенных задач - сразу возвращаем сообщение
	if len(pendingLinks) == 0 && len(pendingNums) == 0 {
		slog.Info("No unfinished work found")
		return &models.AllUnfinishedWork{
			Pdfs: []models.ListOfProcessedLinks{
				{
					Description: "No unfinished work found",
				},
			},
		}
	}

	// Обрабатываем задачи только если они есть
	for i, linkSet := range pendingLinks {
		slog.Info("Processing link set", "index", i, "links_count", len(linkSet.Links))
		processed := l.processLinks(linkSet)
		if processed != nil {
			result.Links = append(result.Links, *processed)
		}
	}

	for i, numSet := range pendingNums {
		slog.Info("Processing PDF set", "index", i, "nums_count", len(numSet.NumsLinks))
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
	// Переносим все данные из надежного хранилища во временное
	allData, err := l.reliable.ReadAllFile()
	if err != nil {
		return &models.ProcessedLinks{
			Answer:  make(models.LinksAnswer),
			ListNum: -1,
		}
	}

	// Выгружаем все данные во временное хранилище
	l.temp.UploadAllData(allData)

	return &models.ProcessedLinks{
		Answer:  make(models.LinksAnswer),
		ListNum: len(*allData),
	}
}

func (l *LinksService) AddLinkSet(set models.SetLinksGet) *models.ProcessedLinks {
	// СИНХРОННО обрабатываем ссылки и сразу возвращаем результат
	hash, err := l.reliable.AddLinksProcessList(&set)
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}

	answer := make(models.LinksAnswer)

	// Проверяем статус каждой ссылки
	for _, url := range set.Links {
		status := l.checkLinkStatus(url)
		answer[url] = status
	}

	// Сохраняем во временное хранилище и получаем номер
	listNum := l.temp.UploadNewData(&models.ProcessedLinks{
		Answer: answer,
	})

	// Сохраняем в надежное хранилище (асинхронно)
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		processed := &models.ProcessedLinks{
			Answer:  answer,
			ListNum: listNum,
		}
		if err := l.reliable.AddNewLinkAllFileTask(processed); err != nil {
			fmt.Printf("Failed to save processed links: %v\n", err)
		}
	}()

	// Немедленно возвращаем результат с номером набора
	err = l.reliable.RemoveLinksProcessByHash(hash)
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
	return &models.ProcessedLinks{
		Answer:  answer,
		ListNum: listNum,
	}
}

func (l *LinksService) GiveLinkAnswer(list models.SetNumsOfLinksGet) *models.ListOfProcessedLinks {
	hash, err := l.reliable.AddNumProcessList(&list)
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
	// СИНХРОННО генерируем PDF и сразу возвращаем результат
	result := l.generatePDF(list)

	// Асинхронно сохраняем в pending и удаляем (для отслеживания незавершенных работ)
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		hash, err := l.reliable.AddNumProcessList(&list)
		if err != nil {
			fmt.Printf("Failed to add to pending: %v\n", err)
			return
		}
		// Немедленно удаляем, так как задача уже выполнена
		l.reliable.RemoveNumsProcessByHash(hash)
	}()
	err = l.reliable.RemoveNumsProcessByHash(hash)
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
	return result
}

func (l *LinksService) processLinks(set models.SetLinksGet) *models.ProcessedLinks {
	answer := make(models.LinksAnswer)

	// Проверяем статус каждой ссылки
	for _, url := range set.Links {
		status := l.checkLinkStatus(url)
		answer[url] = status
	}

	// Сохраняем во временное хранилище и получаем номер
	listNum := l.temp.UploadNewData(&models.ProcessedLinks{
		Answer: answer,
	})

	return &models.ProcessedLinks{
		Answer:  answer,
		ListNum: listNum,
	}
}

func (l *LinksService) checkLinkStatus(url string) string {
	// Добавляем схему если отсутствует
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
	// Получаем данные по номерам
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

	// Создаем PDF
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

// Вспомогательная функция
func hasScheme(url string) bool {
	return len(url) > 7 && (url[0:7] == "http://" || url[0:8] == "https://")
}
