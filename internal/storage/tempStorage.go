package storage

import (
	"fmt"
	"status-links/internal/models"
	"sync"
)

type tempStorageMap struct {
	sets    map[int]models.ProcessedLinks
	lastNum int
	mu      sync.Mutex
}

func NewTempStorage() *tempStorageMap {
	return &tempStorageMap{
		sets:    make(map[int]models.ProcessedLinks),
		lastNum: 0,
	}
}

func (s *tempStorageMap) UploadAllData(bs *[]models.ProcessedLinks) {
	s.mu.Lock()
	defer s.mu.Unlock()
	max := 0
	for i := 0; i < len(*bs); i++ {
		if max < (*bs)[i].ListNum {
			max = (*bs)[i].ListNum
		}
		s.sets[(*bs)[i].ListNum] = (*bs)[i]
	}
	s.lastNum = max
}

func (s *tempStorageMap) UploadNewData(bs *models.ProcessedLinks) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sets[s.lastNum+1] = *bs
	s.lastNum++
	return s.lastNum
}

func (s *tempStorageMap) FindKeys(list *models.SetNumsOfLinksGet) (*[]models.LinksAnswer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bs := make([]models.LinksAnswer, len(list.NumsLinks))
	for i, num := range list.NumsLinks {
		v, ok := s.sets[num]
		if !ok {
			return nil, fmt.Errorf("key %d does not exist", num)
		}
		bs[i] = v.Answer
	}
	return &bs, nil
}

func (s *tempStorageMap) ReturnMaxIndex() int {
	return s.lastNum
}
