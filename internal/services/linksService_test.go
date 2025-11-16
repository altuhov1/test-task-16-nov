package services

import (
	"fmt"
	"status-links/internal/models"
	"sync"
	"testing"
)

type mockTempStorage struct {
	data   map[int]models.ProcessedLinks
	mu     sync.Mutex
	maxInt int
}

func newMockTempStorage() *mockTempStorage {
	return &mockTempStorage{
		data:   make(map[int]models.ProcessedLinks),
		maxInt: 1,
	}
}

func (m *mockTempStorage) UploadAllData(bs *[]models.ProcessedLinks) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range *bs {
		m.data[item.ListNum] = item
	}
}

func (m *mockTempStorage) UploadNewData(bs *models.ProcessedLinks) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	newNum := len(m.data) + 1
	bs.ListNum = newNum
	m.data[newNum] = *bs
	return newNum
}
func (m *mockTempStorage) ReturnMaxIndex() int {
	return m.maxInt
}
func (m *mockTempStorage) FindKeys(list *models.SetNumsOfLinksGet) (*[]models.LinksAnswer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]models.LinksAnswer, 0)
	for _, num := range list.NumsLinks {
		if item, exists := m.data[num]; exists {
			result = append(result, item.Answer)
		} else {
			return nil, fmt.Errorf("key %d does not exist", num)
		}
	}
	return &result, nil
}

type mockReliableStorage struct {
	allData      []models.ProcessedLinks
	pendingLinks []models.SetLinksGet
	pendingNums  []models.SetNumsOfLinksGet
	mu           sync.Mutex
}

func newMockReliableStorage() *mockReliableStorage {
	return &mockReliableStorage{
		allData:      []models.ProcessedLinks{},
		pendingLinks: []models.SetLinksGet{},
		pendingNums:  []models.SetNumsOfLinksGet{},
	}
}

func (m *mockReliableStorage) ReadAllFile() (*[]models.ProcessedLinks, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &m.allData, nil
}

func (m *mockReliableStorage) AddNewLinkPerm(item *models.ProcessedLinks) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allData = append(m.allData, *item)
	return nil
}

func (m *mockReliableStorage) AddLinksProcessList(set *models.SetLinksGet) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pendingLinks = append(m.pendingLinks, *set)
	return "link-hash", nil
}

func (m *mockReliableStorage) AddNumProcessList(set *models.SetNumsOfLinksGet) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pendingNums = append(m.pendingNums, *set)
	return "num-hash", nil
}

func (m *mockReliableStorage) RemoveLinksProcessByHash(hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.pendingLinks) > 0 {
		m.pendingLinks = m.pendingLinks[:len(m.pendingLinks)-1]
	}
	return nil
}

func (m *mockReliableStorage) RemoveNumsProcessByHash(hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.pendingNums) > 0 {
		m.pendingNums = m.pendingNums[:len(m.pendingNums)-1]
	}
	return nil
}

func (m *mockReliableStorage) GetPendingLinksData() ([]models.SetLinksGet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := m.pendingLinks
	m.pendingLinks = []models.SetLinksGet{}
	return result, nil
}

func (m *mockReliableStorage) GetPendingNumsData() ([]models.SetNumsOfLinksGet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := m.pendingNums
	m.pendingNums = []models.SetNumsOfLinksGet{}
	return result, nil
}

func TestLinksService(t *testing.T) {
	t.Run("NewLinksService initializes correctly", func(t *testing.T) {
		tempStorage := newMockTempStorage()
		reliableStorage := newMockReliableStorage()

		service := NewLinksService(tempStorage, reliableStorage)

		if service == nil {
			t.Error("Expected service to be created")
			return
		}
		if service.temp == nil {
			t.Error("Expected temp storage to be set")
		}
		if service.reliable == nil {
			t.Error("Expected reliable storage to be set")
		}
		if service.client == nil {
			t.Error("Expected HTTP client to be set")
		}
	})

	t.Run("AddLinkSet processes links and returns result", func(t *testing.T) {
		tempStorage := newMockTempStorage()
		reliableStorage := newMockReliableStorage()
		service := NewLinksService(tempStorage, reliableStorage)

		set := models.SetLinksGet{
			Links: []string{"https://httpbin.org/status/200", "https://httpbin.org/status/404"},
		}

		result := service.AddLinkSet(set)

		if result == nil {
			t.Error("Expected non-nil result")
			return
		}
		if result.ListNum <= 0 {
			t.Errorf("Expected positive list number, got %d", result.ListNum)
		}
		if len(result.Answer) != 2 {
			t.Errorf("Expected 2 links in answer, got %d", len(result.Answer))
		}

		service.WaitForCompletion()
	})

	t.Run("GiveLinkAnswer generates PDF for existing data", func(t *testing.T) {
		tempStorage := newMockTempStorage()
		reliableStorage := newMockReliableStorage()
		service := NewLinksService(tempStorage, reliableStorage)

		set := models.SetLinksGet{
			Links: []string{"https://example.com"},
		}
		addResult := service.AddLinkSet(set)

		pdfRequest := models.SetNumsOfLinksGet{
			NumsLinks: []int{addResult.ListNum},
		}

		pdfResult, _ := service.GiveLinkAnswer(pdfRequest)

		if pdfResult == nil {
			t.Error("Expected non-nil PDF result")
			return
		}
		if pdfResult.Description == "" {
			t.Error("Expected description in PDF result")
		}
		if len(pdfResult.PDF) == 0 {
			t.Error("Expected non-empty PDF data")
		}

		service.WaitForCompletion()
	})

	t.Run("UploadAllUnfinishedWork with no pending work", func(t *testing.T) {
		tempStorage := newMockTempStorage()
		reliableStorage := newMockReliableStorage()
		service := NewLinksService(tempStorage, reliableStorage)

		result := service.UploadAllUnfinishedWork()

		if result == nil {
			t.Error("Expected non-nil result")
			return
		}
		if len(result.Links) != 0 {
			t.Errorf("Expected no links, got %d", len(result.Links))
		}
		if len(result.Pdfs) == 0 {
			t.Error("Expected at least one PDF info")
			return
		}
		if result.Pdfs[0].Description != "No unfinished work found" {
			t.Errorf("Unexpected description: %s", result.Pdfs[0].Description)
		}
	})

	t.Run("checkLinkStatus handles different URLs", func(t *testing.T) {
		tempStorage := newMockTempStorage()
		reliableStorage := newMockReliableStorage()
		service := NewLinksService(tempStorage, reliableStorage)

		status1 := service.checkLinkStatus("https://httpbin.org/status/200")
		if status1 != "available" {
			t.Errorf("Expected available for 200 status, got %s", status1)
		}

		status2 := service.checkLinkStatus("httpbin.org/status/200")
		if status2 != "available" {
			t.Errorf("Expected available for URL without scheme, got %s", status2)
		}
	})

	t.Run("hasScheme correctly detects URL schemes", func(t *testing.T) {
		tests := []struct {
			url      string
			expected bool
		}{
			{"https://example.com", true},
			{"http://example.com", true},
			{"example.com", false},
			{"ftp://example.com", false},
			{"https://a", true},
			{"http://a", true},
			{"", false},
		}

		for _, test := range tests {
			result := hasScheme(test.url)
			if result != test.expected {
				t.Errorf("hasScheme(%q) = %v, expected %v", test.url, result, test.expected)
			}
		}
	})

	t.Run("generatePDF with empty data returns appropriate message", func(t *testing.T) {
		tempStorage := newMockTempStorage()
		reliableStorage := newMockReliableStorage()
		service := NewLinksService(tempStorage, reliableStorage)

		request := models.SetNumsOfLinksGet{
			NumsLinks: []int{999},
		}

		result := service.generatePDF(request)

		if result == nil {
			t.Error("Expected non-nil result")
			return
		}
		expectedDescription := "Error finding keys: key 999 does not exist"
		if result.Description != expectedDescription {
			t.Errorf("Unexpected description: %s, expected: %s", result.Description, expectedDescription)
		}
		if len(result.PDF) != 0 {
			t.Error("Expected empty PDF for no data")
		}
	})

	t.Run("generatePDF with valid data returns PDF", func(t *testing.T) {
		tempStorage := newMockTempStorage()
		reliableStorage := newMockReliableStorage()
		service := NewLinksService(tempStorage, reliableStorage)

		set := models.SetLinksGet{
			Links: []string{"https://example.com"},
		}
		addResult := service.AddLinkSet(set)

		request := models.SetNumsOfLinksGet{
			NumsLinks: []int{addResult.ListNum},
		}

		result := service.generatePDF(request)

		if result == nil {
			t.Error("Expected non-nil result")
			return
		}
		if result.Description != "PDF report generated successfully" {
			t.Errorf("Unexpected description: %s", result.Description)
		}
		if len(result.PDF) == 0 {
			t.Error("Expected non-empty PDF data")
		}

		service.WaitForCompletion()
	})

}
