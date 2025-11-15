package storage

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"status-links/internal/models"
	"sync"
)

type ProcessTasksLinks struct {
	Data models.SetLinksGet `json:"data"`
	Hash string             `json:"hash"`
}
type ProcessTasksNums struct {
	Data models.SetNumsOfLinksGet `json:"data"`
	Hash string                   `json:"hash"`
}
type AllTasksNums struct {
	DataAn  []models.ProcessedLinks `json:"processed_data"`
	LastNum int                     `json:"lastNum"`
}
type reliableStorageJsonFile struct {
	NameFileAllTasks          string
	NameFileProcessTasksLinks string
	NameFileProcessTasksNums  string
	muAllTasks                sync.Mutex
	muTasksLinks              sync.Mutex
	muTasksNums               sync.Mutex
}

func NewReliableStorage(NameFileAllTasks string, NameFileProcessTasksLinks string, NameFileProcessTasksNums string) *reliableStorageJsonFile {
	s := &reliableStorageJsonFile{
		NameFileAllTasks:          NameFileAllTasks,
		NameFileProcessTasksLinks: NameFileProcessTasksLinks,
		NameFileProcessTasksNums:  NameFileProcessTasksNums,
	}
	if _, err := os.Stat(s.NameFileAllTasks); os.IsNotExist(err) {
		s.writeJSON(s.NameFileAllTasks, &AllTasksNums{DataAn: []models.ProcessedLinks{}, LastNum: 0})
	}
	if _, err := os.Stat(s.NameFileProcessTasksLinks); os.IsNotExist(err) {
		s.writeJSON(s.NameFileProcessTasksLinks, []ProcessTasksLinks{})
	}
	if _, err := os.Stat(s.NameFileProcessTasksNums); os.IsNotExist(err) {
		s.writeJSON(s.NameFileProcessTasksNums, []ProcessTasksNums{})
	}
	return s
}
func (s *reliableStorageJsonFile) writeJSON(filename string, data interface{}) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(data)
}
func (s *reliableStorageJsonFile) ReadAllFile() (*[]models.ProcessedLinks, error) {
	s.muAllTasks.Lock()
	defer s.muAllTasks.Unlock()
	file, err := os.Open(s.NameFileAllTasks)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to open storage file: %w", err)
		}
		empty := make([]models.ProcessedLinks, 0)
		return &empty, nil
	}
	defer file.Close()

	var data AllTasksNums
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode storage file %q: %w", s.NameFileAllTasks, err)
	}
	return &data.DataAn, nil
}
func (s *reliableStorageJsonFile) AddNewLinkAllFileTask(item *models.ProcessedLinks) error {
	s.muAllTasks.Lock()
	defer s.muAllTasks.Unlock()
	file, err := os.Open(s.NameFileAllTasks)
	if err != nil {
		if os.IsNotExist(err) {
			data := AllTasksNums{
				DataAn:  []models.ProcessedLinks{*item},
				LastNum: 1,
			}
			return s.writeAllTasks(&data)
		}
		return err
	}
	defer file.Close()

	var data AllTasksNums
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return err
	}

	data.DataAn = append(data.DataAn, *item)
	data.LastNum++

	return s.writeAllTasks(&data)
}

func (s *reliableStorageJsonFile) writeAllTasks(data *AllTasksNums) error {
	file, err := os.Create(s.NameFileAllTasks)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(data)
}

func joinStringsToBytes(strings []string) []byte {
	var result []byte
	for _, s := range strings {
		result = append(result, s...)
	}
	return result
}

func (s *reliableStorageJsonFile) AddLinksProcessList(masLinks *models.SetLinksGet) (string, error) {
	s.muTasksLinks.Lock()
	defer s.muTasksLinks.Unlock()
	var data []ProcessTasksLinks

	file, err := os.Open(s.NameFileProcessTasksLinks)
	if err != nil {
		if os.IsNotExist(err) {
			data = []ProcessTasksLinks{}

		} else {
			return "", err
		}
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&data)
	if err != nil {
		if err == io.EOF {
			data = []ProcessTasksLinks{}
		} else {
			return "", err
		}
	}

	NewNode := ProcessTasksLinks{
		Data: *masLinks,
		Hash: fmt.Sprintf("%x", md5.Sum(joinStringsToBytes(masLinks.Links))),
	}
	data = append(data, NewNode)

	file, err = os.Create(s.NameFileProcessTasksLinks)
	if err != nil {
		return "", err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	err = encoder.Encode(data)
	if err != nil {
		return "", err
	}

	return NewNode.Hash, nil
}

func intArrayToBytes(arr []int) []byte {
	result := make([]byte, len(arr)*8)
	for i, v := range arr {
		for j := 0; j < 8; j++ {
			result[i*8+j] = byte(v >> uint(j*8))
		}
	}
	return result
}

func (s *reliableStorageJsonFile) AddNumProcessList(masLinks *models.SetNumsOfLinksGet) (string, error) {
	s.muTasksNums.Lock()
	defer s.muTasksNums.Unlock()
	var data []ProcessTasksNums

	file, err := os.Open(s.NameFileProcessTasksNums)
	if err != nil {
		if os.IsNotExist(err) {
			data = []ProcessTasksNums{}
		} else {
			file, err := os.Create(s.NameFileAllTasks)
			if err != nil {
				return "", fmt.Errorf("failed to create storage file: %w", err)
			}
			file.Close()
			return "", err
		}
	} else {
		defer file.Close()

		decoder := json.NewDecoder(file)
		err = decoder.Decode(&data)
		if err != nil {
			if err == io.EOF {
				data = []ProcessTasksNums{}
			} else {
				return "", err
			}
		}
	}

	NewNode := ProcessTasksNums{
		Data: *masLinks,
		Hash: fmt.Sprintf("%x", md5.Sum(intArrayToBytes(masLinks.NumsLinks))),
	}
	data = append(data, NewNode)

	file, err = os.Create(s.NameFileProcessTasksNums)
	if err != nil {
		return "", err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	err = encoder.Encode(data)
	if err != nil {
		return "", err
	}

	return NewNode.Hash, nil
}

func (s *reliableStorageJsonFile) RemoveLinksProcessByHash(targetHash string) error {
	s.muTasksLinks.Lock()
	defer s.muTasksLinks.Unlock()
	file, err := os.Open(s.NameFileProcessTasksLinks)
	if err != nil {
		return err
	}
	defer file.Close()

	var data []ProcessTasksLinks
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return err
	}

	found := false
	filtered := make([]ProcessTasksLinks, 0, len(data))
	for _, item := range data {
		if item.Hash != targetHash {
			filtered = append(filtered, item)
		} else {
			found = true
		}
	}

	if !found {
		return os.ErrNotExist
	}

	file, err = os.Create(s.NameFileProcessTasksLinks)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(filtered)
}

func (s *reliableStorageJsonFile) RemoveNumsProcessByHash(targetHash string) error {
	s.muTasksNums.Lock()
	defer s.muTasksNums.Unlock()
	file, err := os.Open(s.NameFileProcessTasksNums)
	if err != nil {
		return err
	}
	defer file.Close()

	var data []ProcessTasksNums
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return err
	}

	found := false
	filtered := make([]ProcessTasksNums, 0, len(data))
	for _, item := range data {
		if item.Hash != targetHash {
			filtered = append(filtered, item)
		} else {
			found = true
		}
	}

	if !found {
		return os.ErrNotExist
	}

	file, err = os.Create(s.NameFileProcessTasksNums)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(filtered)
}
func (s *reliableStorageJsonFile) getPendingLinks() ([]ProcessTasksLinks, error) {
	s.muTasksLinks.Lock()
	defer s.muTasksLinks.Unlock()

	file, err := os.Open(s.NameFileProcessTasksLinks)
	if err != nil {
		if os.IsNotExist(err) {
			return []ProcessTasksLinks{}, nil
		}
		return nil, fmt.Errorf("не удалось открыть файл ожидающих ссылок: %w", err)
	}
	defer file.Close()

	var tasks []ProcessTasksLinks
	if err := json.NewDecoder(file).Decode(&tasks); err != nil {
		if err == io.EOF {
			return []ProcessTasksLinks{}, nil
		}
		return nil, fmt.Errorf("ошибка десериализации файла ожидающих ссылок: %w", err)
	}

	return tasks, nil
}

func (s *reliableStorageJsonFile) getPendingNums() ([]ProcessTasksNums, error) {
	s.muTasksNums.Lock()
	defer s.muTasksNums.Unlock()

	file, err := os.Open(s.NameFileProcessTasksNums)
	if err != nil {
		if os.IsNotExist(err) {
			return []ProcessTasksNums{}, nil
		}
		return nil, fmt.Errorf("не удалось открыть файл ожидающих номеров: %w", err)
	}
	defer file.Close()

	var tasks []ProcessTasksNums
	if err := json.NewDecoder(file).Decode(&tasks); err != nil {
		if err == io.EOF {
			return []ProcessTasksNums{}, nil
		}
		return nil, fmt.Errorf("ошибка десериализации файла ожидающих номеров: %w", err)
	}

	return tasks, nil
}

func (s *reliableStorageJsonFile) GetPendingLinksData() ([]models.SetLinksGet, error) {
	tasks, err := s.getPendingLinks()
	if err != nil {
		return nil, err
	}

	result := make([]models.SetLinksGet, len(tasks))
	for i, task := range tasks {
		result[i] = task.Data
	}

	if len(tasks) > 0 {
		if err := s.writeJSON(s.NameFileProcessTasksLinks, []ProcessTasksLinks{}); err != nil {
			return nil, fmt.Errorf("failed to clear links file: %w", err)
		}
	}

	return result, nil
}

func (s *reliableStorageJsonFile) GetPendingNumsData() ([]models.SetNumsOfLinksGet, error) {
	tasks, err := s.getPendingNums()
	if err != nil {
		return nil, err
	}

	result := make([]models.SetNumsOfLinksGet, len(tasks))
	for i, task := range tasks {
		result[i] = task.Data
	}

	if len(tasks) > 0 {
		if err := s.writeJSON(s.NameFileProcessTasksNums, []ProcessTasksNums{}); err != nil {
			return nil, fmt.Errorf("failed to clear nums file: %w", err)
		}
	}

	return result, nil
}
