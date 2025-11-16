package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"status-links/internal/models"
	"status-links/internal/services"
)

type Handler struct {
	LinkService services.LinkProcessor
}

func NewHandler(linkService services.LinkProcessor) (*Handler, error) {
	return &Handler{
		LinkService: linkService,
	}, nil
}

func (h *Handler) LoadUnfinishedWork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	unfinishedWork := h.LinkService.UploadAllUnfinishedWork()

	var pdfsWithData []models.ListOfProcessedLinks
	for _, pdf := range unfinishedWork.Pdfs {
		if len(pdf.PDF) > 0 {
			pdfsWithData = append(pdfsWithData, pdf)
		}
	}
	fmt.Println(len(pdfsWithData), len(unfinishedWork.Links))
	if len(pdfsWithData) > 0 || len(unfinishedWork.Links) > 0 {
		h.createZipResponse(w, pdfsWithData, unfinishedWork.Links, "unfinished_work")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(unfinishedWork)
}

func (h *Handler) SaveNewUrls(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req models.SetLinksGet
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("Invalid JSON in link request", "error", err)
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if len(req.Links) == 0 {
		http.Error(w, `{"error":"no links provided"}`, http.StatusBadRequest)
		return
	}

	if len(req.Links) > 100 {
		http.Error(w, `{"error":"too many links, maximum 100"}`, http.StatusBadRequest)
		return
	}

	result := h.LinkService.AddLinkSet(req)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"links":     result.Answer,
		"links_num": result.ListNum,
	}

	json.NewEncoder(w).Encode(response)
}

func (h *Handler) LoadUrls(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req models.SetNumsOfLinksGet
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("Invalid JSON in link request", "error", err)
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if len(req.NumsLinks) == 0 {
		http.Error(w, `{"error":"no link numbers provided"}`, http.StatusBadRequest)
		return
	}

	if len(req.NumsLinks) > 50 {
		http.Error(w, `{"error":"too many link numbers, maximum 50"}`, http.StatusBadRequest)
		return
	}

	result, err := h.LinkService.GiveLinkAnswer(req)
	if err == services.ErrTooBigIndex {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid_index",
			"message": "One or more link numbers are out of range",
		})
		return
	}

	if len(result.PDF) > 0 {
		h.sendPDFResponse(w, result.PDF, "links_report")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"message": result.Description,
		"status":  "processing",
	})
}

func (h *Handler) sendPDFResponse(w http.ResponseWriter, pdfData []byte, filename string) {
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.pdf"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfData)))
	w.WriteHeader(http.StatusOK)
	w.Write(pdfData)
}

func (h *Handler) createZipResponse(w http.ResponseWriter, pdfs []models.ListOfProcessedLinks, links []models.ProcessedLinks, baseFilename string) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	for i, pdf := range pdfs {
		if len(pdf.PDF) > 0 {
			filename := fmt.Sprintf("report_%d.pdf", i+1)
			writer, err := zipWriter.Create(filename)
			if err != nil {
				slog.Error("Failed to create PDF in ZIP", "error", err)
				continue
			}
			if _, err := writer.Write(pdf.PDF); err != nil {
				slog.Error("Failed to write PDF to ZIP", "error", err)
			} else {
				slog.Info("PDF added to ZIP", "filename", filename, "size", len(pdf.PDF))
			}
		}
	}

	if len(links) > 0 {
		writer, err := zipWriter.Create("links_info.txt")
		if err != nil {
			slog.Error("Failed to create links file in ZIP", "error", err)
		} else {
			content := "LINKS STATUS REPORT\n===================\n\n"

			for i, linkSet := range links {
				content += fmt.Sprintf("Link Set #%d (ID: %d):\n", i+1, linkSet.ListNum)
				content += "----------------------------------------\n"

				for url, status := range linkSet.Answer {
					content += fmt.Sprintf("  %s - %s\n", url, status)
				}
				content += "\n"
			}

			if _, err := writer.Write([]byte(content)); err != nil {
				slog.Error("Failed to write links to ZIP", "error", err)
			} else {
				slog.Info("Links info added to ZIP", "size", len(content))
			}
		}
	}

	if err := zipWriter.Close(); err != nil {
		slog.Error("Failed to close ZIP", "error", err)
		http.Error(w, "Failed to create ZIP archive", http.StatusInternalServerError)
		return
	}

	debugFilename := "debug_unfinished_work.zip"
	if err := os.WriteFile(debugFilename, buf.Bytes(), 0644); err != nil {
		slog.Error("Failed to save debug ZIP", "error", err)
	}

	slog.Info("ZIP created successfully", "size", buf.Len())

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, baseFilename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))
	w.WriteHeader(http.StatusOK)

	if _, err := buf.WriteTo(w); err != nil {
		slog.Error("Failed to send ZIP", "error", err)
	}
}
