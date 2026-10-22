// Package api 覆盖回写 HTTP 接入。
package api

import (
	"net/http"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
)

func (h *runHandler) syncCoverage(w http.ResponseWriter, r *http.Request) {
	id, ok := runID(w, r)
	if !ok {
		return
	}
	if _, ok := h.findAndAuthorize(w, r, id, "exec"); !ok {
		return
	}
	updated, err := h.svc.SyncCoverage(r.Context(), id)
	if err != nil {
		httpx.WriteErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]int{"updated": updated})
}
