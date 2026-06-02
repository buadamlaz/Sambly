package handlers

import (
	"net/http"
	"strconv"
)

const logsPerPage = 25

type logsPageData struct {
	Entries    any
	Page       int
	TotalPages int
	Total      int
	Pages      []int
	HasPrev    bool
	HasNext    bool
}

func (h *Handler) handleLogs(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requireSession(r)
	if !ok {
		h.redirectLogin(w, r)
		return
	}

	total, _ := h.db.GetAuditLogsCount()
	totalPages := (total + logsPerPage - 1) / logsPerPage
	if totalPages < 1 {
		totalPages = 1
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	offset := (page - 1) * logsPerPage
	entries, err := h.db.GetAuditLogsPaged(logsPerPage, offset)

	pd := PageData{
		Title:      "Audit Log — Sambly",
		Username:   sess.Username,
		CSRF:       sess.CSRFToken,
		ActivePage: "logs",
		Data: logsPageData{
			Entries:    entries,
			Page:       page,
			TotalPages: totalPages,
			Total:      total,
			Pages:      pageWindow(page, totalPages),
			HasPrev:    page > 1,
			HasNext:    page < totalPages,
		},
	}
	if err != nil {
		pd.FlashErr = "Failed to load logs: " + err.Error()
	}
	h.render(w, "logs", pd)
}

// pageWindow returns up to 7 page numbers centered around current page.
func pageWindow(current, total int) []int {
	const window = 7
	start := current - window/2
	if start < 1 {
		start = 1
	}
	end := start + window - 1
	if end > total {
		end = total
		start = end - window + 1
		if start < 1 {
			start = 1
		}
	}
	pages := make([]int, 0, end-start+1)
	for i := start; i <= end; i++ {
		pages = append(pages, i)
	}
	return pages
}
