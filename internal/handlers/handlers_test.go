package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"status-links/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockLinkProcessor struct {
	mock.Mock
}

func (m *MockLinkProcessor) UploadAllUnfinishedWork() *models.AllUnfinishedWork {
	args := m.Called()
	return args.Get(0).(*models.AllUnfinishedWork)
}

func (m *MockLinkProcessor) AddLinkSet(req models.SetLinksGet) *models.ProcessedLinks {
	args := m.Called(req)
	return args.Get(0).(*models.ProcessedLinks)
}

func (m *MockLinkProcessor) GiveLinkAnswer(req models.SetNumsOfLinksGet) (*models.ListOfProcessedLinks, error) {
	args := m.Called(req)
	return args.Get(0).(*models.ListOfProcessedLinks), nil
}

func (m *MockLinkProcessor) WaitForCompletion() {
	m.Called()
}

func TestSaveNewUrls_Success(t *testing.T) {
	mockService := new(MockLinkProcessor)

	mockService.On("AddLinkSet", mock.AnythingOfType("models.SetLinksGet")).
		Return(&models.ProcessedLinks{
			Answer: models.LinksAnswer{
				"https://example.com": "processed",
				"https://google.com":  "pending",
			},
			ListNum: 123,
		})

	handler, _ := NewHandler(mockService)

	requestBody := models.SetLinksGet{
		Links: []string{"https://example.com", "https://google.com"},
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.Encode(requestBody)

	req := httptest.NewRequest("POST", "/save-urls", &buf)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	handler.SaveNewUrls(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var response map[string]interface{}
	decoder := json.NewDecoder(rr.Body)
	err := decoder.Decode(&response)
	assert.NoError(t, err)

	assert.Contains(t, response, "links")
	assert.Contains(t, response, "links_num")
	assert.Equal(t, float64(123), response["links_num"])

	mockService.AssertCalled(t, "AddLinkSet", mock.AnythingOfType("models.SetLinksGet"))
}

func TestSaveNewUrls_InvalidJSON(t *testing.T) {
	mockService := new(MockLinkProcessor)
	handler, _ := NewHandler(mockService)

	req := httptest.NewRequest("POST", "/save-urls", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.SaveNewUrls(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errorResponse map[string]string
	decoder := json.NewDecoder(rr.Body)
	decoder.Decode(&errorResponse)

	assert.Contains(t, errorResponse["error"], "invalid JSON")

	mockService.AssertNotCalled(t, "AddLinkSet")
}

func TestSaveNewUrls_TooManyLinks(t *testing.T) {
	mockService := new(MockLinkProcessor)
	handler, _ := NewHandler(mockService)

	links := make([]string, 101)
	for i := range links {
		links[i] = "https://example.com"
	}

	requestBody := models.SetLinksGet{
		Links: links,
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.Encode(requestBody)

	req := httptest.NewRequest("POST", "/save-urls", &buf)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.SaveNewUrls(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errorResponse map[string]string
	decoder := json.NewDecoder(rr.Body)
	decoder.Decode(&errorResponse)

	assert.Contains(t, errorResponse["error"], "too many links")

	mockService.AssertNotCalled(t, "AddLinkSet")
}

func TestLoadUnfinishedWork_OnlyLinks_ReturnsZip(t *testing.T) {
	mockService := new(MockLinkProcessor)
	handler, _ := NewHandler(mockService)
	mockService.On("UploadAllUnfinishedWork").
		Return(&models.AllUnfinishedWork{
			Pdfs: []models.ListOfProcessedLinks{},
			Links: []models.ProcessedLinks{
				{
					Answer: models.LinksAnswer{
						"https://test.com": "completed",
					},
					ListNum: 1,
				},
			},
		})

	req := httptest.NewRequest("GET", "/unfinished-work", nil)
	rr := httptest.NewRecorder()

	handler.LoadUnfinishedWork(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/zip", rr.Header().Get("Content-Type"))

	mockService.AssertCalled(t, "UploadAllUnfinishedWork")

	os.Remove("debug_unfinished_work.zip")
}
