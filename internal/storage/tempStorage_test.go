package storage

import (
	"fmt"
	"status-links/internal/models"
	"sync"
	"testing"
)

func TestTempStorageMap(t *testing.T) {
	t.Run("UploadNewData adds data and returns correct number", func(t *testing.T) {
		storage := NewTempStorage()

		data := &models.ProcessedLinks{
			Answer: models.LinksAnswer{
				"https://example.com": "available",
			},
			ListNum: 0,
		}

		num := storage.UploadNewData(data)

		if num != 1 {
			t.Errorf("Expected first upload to return 1, got %d", num)
		}
		if storage.lastNum != 1 {
			t.Errorf("Expected lastNum to be 1, got %d", storage.lastNum)
		}
		if len(storage.sets) != 1 {
			t.Errorf("Expected 1 item in sets, got %d", len(storage.sets))
		}

		stored, exists := storage.sets[1]
		if !exists {
			t.Error("Expected data to be stored with key 1")
		}
		if stored.Answer["https://example.com"] != "available" {
			t.Errorf("Expected stored data to match input")
		}
	})

	t.Run("FindKeys returns correct data", func(t *testing.T) {
		storage := NewTempStorage()

		batch := []models.ProcessedLinks{
			{
				Answer:  models.LinksAnswer{"link1": "available"},
				ListNum: 1,
			},
			{
				Answer:  models.LinksAnswer{"link2": "unavailable"},
				ListNum: 2,
			},
		}
		storage.UploadAllData(&batch)

		request := &models.SetNumsOfLinksGet{
			NumsLinks: []int{1, 2},
		}

		result, err := storage.FindKeys(request)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if len(*result) != 2 {
			t.Errorf("Expected 2 results, got %d", len(*result))
		}

		if (*result)[0]["link1"] != "available" {
			t.Error("Expected first result to contain link1 with status 'available'")
		}
		if (*result)[1]["link2"] != "unavailable" {
			t.Error("Expected second result to contain link2 with status 'unavailable'")
		}
	})

	t.Run("UploadNewData increments counter correctly", func(t *testing.T) {
		storage := NewTempStorage()

		batch := []models.ProcessedLinks{
			{
				Answer:  models.LinksAnswer{"link1": "available"},
				ListNum: 5,
			},
		}
		storage.UploadAllData(&batch)

		newData := &models.ProcessedLinks{
			Answer: models.LinksAnswer{"newlink": "unavailable"},
		}

		num := storage.UploadNewData(newData)

		if num != 6 {
			t.Errorf("Expected new upload to return 6, got %d", num)
		}
		if storage.lastNum != 6 {
			t.Errorf("Expected lastNum to be 6, got %d", storage.lastNum)
		}
	})

	t.Run("Mixed statuses work correctly", func(t *testing.T) {
		storage := NewTempStorage()

		data := &models.ProcessedLinks{
			Answer: models.LinksAnswer{
				"https://google.com":  "available",
				"https://invalid.com": "unavailable",
				"http://example.com":  "available",
			},
		}

		num := storage.UploadNewData(data)

		result, err := storage.FindKeys(&models.SetNumsOfLinksGet{NumsLinks: []int{num}})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		answer := (*result)[0]
		if answer["https://google.com"] != "available" {
			t.Error("Expected google.com to be available")
		}
		if answer["https://invalid.com"] != "unavailable" {
			t.Error("Expected invalid.com to be unavailable")
		}
		if answer["http://example.com"] != "available" {
			t.Error("Expected example.com to be available")
		}
	})

	t.Run("Concurrent access works correctly", func(t *testing.T) {
		storage := NewTempStorage()
		var wg sync.WaitGroup

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				data := &models.ProcessedLinks{
					Answer: models.LinksAnswer{fmt.Sprintf("link%d", n): "available"},
				}
				storage.UploadNewData(data)
			}(i)
		}

		wg.Wait()

		if storage.lastNum != 100 {
			t.Errorf("Expected lastNum to be 100, got %d", storage.lastNum)
		}
		if len(storage.sets) != 100 {
			t.Errorf("Expected 100 items, got %d", len(storage.sets))
		}
	})
}
