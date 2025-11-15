package models

type SetLinksGet struct {
	Links []string `json:"links"`
}

type SetNumsOfLinksGet struct {
	NumsLinks []int `json:"links_list"`
}

type LinksAnswer map[string]string

type ProcessedLinks struct {
	Answer  LinksAnswer `json:"links"`
	ListNum int         `json:"links_num"`
}

type ListOfProcessedLinks struct {
	Description string `json:"description,omitempty"`
	PDF         []byte `json:"-"`
}

type AllUnfinishedWork struct {
	Pdfs  []ListOfProcessedLinks `json:"pdfs,omitempty"`
	Links []ProcessedLinks       `json:"links,omitempty"`
}
