package storage

import (
	"os"
	"status-links/internal/models"
	"testing"
)

func TestReliableStorageJsonFile(t *testing.T) {
	tempFiles := []string{
		"test_all_tasks.json",
		"test_process_links.json",
		"test_process_nums.json",
	}

	defer func() {
		for _, file := range tempFiles {
			os.Remove(file)
		}
	}()

	t.Run("AddLinksProcessList and RemoveLinksProcessByHash", func(t *testing.T) {
		storage := NewReliableStorage(tempFiles[0], tempFiles[1], tempFiles[2])

		set := models.SetLinksGet{
			Links: []string{"https://example.com", "https://google.com"},
		}

		hash, err := storage.AddLinksProcessList(&set)
		if err != nil {
			t.Errorf("Unexpected error adding links: %v", err)
		}
		if hash == "" {
			t.Error("Expected non-empty hash")
		}

		err = storage.RemoveLinksProcessByHash(hash)
		if err != nil {
			t.Errorf("Unexpected error removing links: %v", err)
		}
	})

	t.Run("AddNumProcessList and RemoveNumsProcessByHash", func(t *testing.T) {
		storage := NewReliableStorage(tempFiles[0], tempFiles[1], tempFiles[2])

		set := models.SetNumsOfLinksGet{
			NumsLinks: []int{1, 2, 3},
		}

		hash, err := storage.AddNumProcessList(&set)
		if err != nil {
			t.Errorf("Unexpected error adding nums: %v", err)
		}
		if hash == "" {
			t.Error("Expected non-empty hash")
		}

		err = storage.RemoveNumsProcessByHash(hash)
		if err != nil {
			t.Errorf("Unexpected error removing nums: %v", err)
		}
	})

	t.Run("AddNewLinkPerm and ReadAllFile", func(t *testing.T) {
		storage := NewReliableStorage(tempFiles[0], tempFiles[1], tempFiles[2])

		item := &models.ProcessedLinks{
			Answer: models.LinksAnswer{
				"https://test.com": "available",
			},
			ListNum: 1,
		}

		err := storage.AddNewLinkPerm(item)
		if err != nil {
			t.Errorf("Unexpected error adding task: %v", err)
		}

		data, err := storage.ReadAllFile()
		if err != nil {
			t.Errorf("Unexpected error reading file: %v", err)
		}

		if len(*data) != 1 {
			t.Errorf("Expected 1 item, got %d", len(*data))
		}

		if (*data)[0].Answer["https://test.com"] != "available" {
			t.Error("Expected stored data to match")
		}
	})

	t.Run("GetPendingLinksData with empty storage", func(t *testing.T) {
		storage := NewReliableStorage(tempFiles[0], tempFiles[1], tempFiles[2])

		pending, err := storage.GetPendingLinksData()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if len(pending) != 0 {
			t.Errorf("Expected empty pending links, got %d", len(pending))
		}
	})

	t.Run("GetPendingNumsData with empty storage", func(t *testing.T) {
		storage := NewReliableStorage(tempFiles[0], tempFiles[1], tempFiles[2])

		pending, err := storage.GetPendingNumsData()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if len(pending) != 0 {
			t.Errorf("Expected empty pending nums, got %d", len(pending))
		}
	})

	t.Run("Remove non-existent hash returns error", func(t *testing.T) {
		storage := NewReliableStorage(tempFiles[0], tempFiles[1], tempFiles[2])

		err := storage.RemoveLinksProcessByHash("non-existent-hash")
		if err == nil {
			t.Error("Expected error when removing non-existent hash")
		}

		err = storage.RemoveNumsProcessByHash("non-existent-hash")
		if err == nil {
			t.Error("Expected error when removing non-existent hash")
		}
	})

	t.Run("Multiple operations work correctly", func(t *testing.T) {
		storage := NewReliableStorage(tempFiles[0], tempFiles[1], tempFiles[2])

		set1 := models.SetLinksGet{Links: []string{"link1", "link2"}}
		set2 := models.SetLinksGet{Links: []string{"link3"}}

		hash1, err := storage.AddLinksProcessList(&set1)
		if err != nil {
			t.Errorf("Error adding set1: %v", err)
		}

		hash2, err := storage.AddLinksProcessList(&set2)
		if err != nil {
			t.Errorf("Error adding set2: %v", err)
		}

		err = storage.RemoveLinksProcessByHash(hash1)
		if err != nil {
			t.Errorf("Error removing hash1: %v", err)
		}

		tasks, err := storage.getPendingLinks()
		if err != nil {
			t.Errorf("Error getting pending: %v", err)
		}

		if len(tasks) != 1 {
			t.Errorf("Expected 1 pending task after removal, got %d", len(tasks))
		}

		if len(tasks) > 0 && tasks[0].Hash != hash2 {
			t.Errorf("Expected remaining task to have hash %s, got %s", hash2, tasks[0].Hash)
		}
	})

	t.Run("Hash generation is consistent", func(t *testing.T) {
		storage := NewReliableStorage(tempFiles[0], tempFiles[1], tempFiles[2])

		set := models.SetLinksGet{
			Links: []string{"https://example.com", "https://google.com"},
		}

		hash1, err := storage.AddLinksProcessList(&set)
		if err != nil {
			t.Errorf("Error first add: %v", err)
		}

		hash2, err := storage.AddLinksProcessList(&set)
		if err != nil {
			t.Errorf("Error second add: %v", err)
		}

		if hash1 != hash2 {
			t.Errorf("Expected same hash for same input, got %s and %s", hash1, hash2)
		}
	})
}
