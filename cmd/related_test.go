package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/daptin/daptin-cli/client"
)

func TestFindAllRelatedCollectsBoundedPages(t *testing.T) {
	server := relatedPaginationServer(t, 5)
	defer server.Close()
	appCtx := &AppContext{Client: client.New(server.URL, "", false)}

	rows, err := findAllRelated(appCtx, "world", "world-ref", "usergroup_id", 2, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(rows))
	}
}

func TestFindAllRelatedFailsBeforeRenderingWhenLimitExceeded(t *testing.T) {
	server := relatedPaginationServer(t, 5)
	defer server.Close()
	appCtx := &AppContext{Client: client.New(server.URL, "", false)}

	rows, err := findAllRelated(appCtx, "world", "world-ref", "usergroup_id", 2, 2)
	if err == nil || !strings.Contains(err.Error(), "exceed --max-pages=2") {
		t.Fatalf("expected truncation error, got rows=%d err=%v", len(rows), err)
	}
	if rows != nil {
		t.Fatalf("expected no printable rows on truncation, got %d", len(rows))
	}
}

func relatedPaginationServer(t *testing.T, total int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageSize, err := strconv.Atoi(r.URL.Query().Get("page[size]"))
		if err != nil || pageSize <= 0 {
			t.Fatalf("invalid page size: %q", r.URL.Query().Get("page[size]"))
		}
		pageNumber, err := strconv.Atoi(r.URL.Query().Get("page[number]"))
		if err != nil || pageNumber <= 0 {
			t.Fatalf("invalid page number: %q", r.URL.Query().Get("page[number]"))
		}
		start := (pageNumber - 1) * pageSize
		end := start + pageSize
		if end > total {
			end = total
		}
		if start > total {
			start = total
		}
		parts := make([]string, 0, end-start)
		for i := start; i < end; i++ {
			parts = append(parts, fmt.Sprintf(`{"id":"relation-%d","attributes":{"reference_id":"relation-%d","permission":7}}`, i, i))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"data":[%s]}`, strings.Join(parts, ","))
	}))
}
