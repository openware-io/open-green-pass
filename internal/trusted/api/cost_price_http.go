package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/iam"
	"github.com/openware-io/open-green-pass/internal/trusted/application"
	trusteddomain "github.com/openware-io/open-green-pass/internal/trusted/domain"
)

// RegisterCostPrices exposes effective-dated model tariffs.
func RegisterCostPrices(mux *http.ServeMux, svc *application.CostPriceService) {
	mux.HandleFunc("GET /cost/prices", func(w http.ResponseWriter, r *http.Request) {
		prices, err := svc.List(r.Context())
		if err != nil {
			httpx.WriteErr(w, err)
			return
		}
		out := make([]map[string]any, 0, len(prices))
		for _, p := range prices {
			out = append(out, map[string]any{
				"id": p.ID, "model": p.Model, "currency": p.Currency,
				"input_unit_price": p.InputUnitPrice, "output_unit_price": p.OutputUnitPrice,
				"cache_read_unit_price": p.CacheReadUnitPrice, "effective_from": p.EffectiveFrom,
			})
		}
		httpx.WriteJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("POST /cost/prices", func(w http.ResponseWriter, r *http.Request) {
		var req application.CreateCostPriceRequest
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.WriteErr(w, err)
			return
		}
		if principal, ok := iam.PrincipalFromContext(r.Context()); ok {
			req.ActorID, _ = strconv.ParseInt(principal.Subject, 10, 64)
		}
		if req.ActorID <= 0 {
			req.ActorID = 1
		}
		price, err := svc.Create(r.Context(), req)
		if err != nil {
			if errors.Is(err, trusteddomain.ErrCostPriceExists) {
				httpx.WriteJSON(w, http.StatusConflict, map[string]string{"error": "cost_price_exists"})
				return
			}
			httpx.WriteErr(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusCreated, map[string]any{
			"id": price.ID, "model": price.Model, "currency": price.Currency,
			"input_unit_price": price.InputUnitPrice, "output_unit_price": price.OutputUnitPrice,
			"cache_read_unit_price": price.CacheReadUnitPrice, "effective_from": price.EffectiveFrom,
		})
	})
}
