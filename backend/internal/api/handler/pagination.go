package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
	// maxPage bounds the page number so the computed offset can never overflow
	// (1_000_000 * 200 = 2e8, far below the 32-bit integer limit) and runaway
	// offsets cannot be requested.
	maxPage = 1_000_000
)

// pageParams is a validated, 1-based page request.
type pageParams struct {
	Page     int
	PageSize int
	Offset   int
}

// parsePageParams reads `page` and `page_size` query parameters. Invalid values
// are rejected; an oversized page_size is clamped to maxPageSize.
func parsePageParams(r *http.Request) (pageParams, error) {
	page := 1
	pageSize := defaultPageSize

	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > maxPage {
			return pageParams{}, fmt.Errorf("page must be an integer between 1 and %d", maxPage)
		}
		page = value
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("page_size")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			return pageParams{}, fmt.Errorf("page_size must be a positive integer")
		}
		if value > maxPageSize {
			value = maxPageSize
		}
		pageSize = value
	}

	return pageParams{Page: page, PageSize: pageSize, Offset: (page - 1) * pageSize}, nil
}

// tableDataPageSize is the Table Data browse default (ADR-010 §6: default 100,
// max 200). It is intentionally larger than the history default because a table
// page is a viewport, not an audit list.
const tableDataPageSize = 100

// parseTablePageParams reads `page`/`page_size` for Table Data browsing with the
// ADR-010 defaults. Invalid values are rejected; an oversized page_size is
// clamped to maxPageSize; page is bounded so the offset cannot overflow.
func parseTablePageParams(r *http.Request) (pageParams, error) {
	page := 1
	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > maxPage {
			return pageParams{}, fmt.Errorf("page must be an integer between 1 and %d", maxPage)
		}
		page = value
	}

	pageSize := tableDataPageSize
	if raw := strings.TrimSpace(r.URL.Query().Get("page_size")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			return pageParams{}, fmt.Errorf("page_size must be a positive integer")
		}
		if value > maxPageSize {
			value = maxPageSize
		}
		pageSize = value
	}

	return pageParams{Page: page, PageSize: pageSize, Offset: (page - 1) * pageSize}, nil
}

// pageMeta builds the pagination metadata placed in the envelope `meta` field.
func pageMeta(page, pageSize, total int) map[string]any {
	totalPages := 0
	if pageSize > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	return map[string]any{
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	}
}
