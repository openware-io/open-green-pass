package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/iam"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
)

// generationErrorBody is the stable error envelope of the generation API. The
// code is the machine-readable contract value (`plan_stale`,
// `idempotency_conflict`, `forbidden`, ...); message is operator-facing.
type generationErrorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func writeGenerationError(w http.ResponseWriter, status int, code, message string) {
	httpx.WriteJSON(w, status, generationErrorBody{Error: code, Message: message})
}

// authorizeGeneration enforces the frozen G55 generation policy:
// plan/create/review/cancel require project `edit`, reads require `view`.
// It fails closed: nil authorizer is only the legacy embedded/test composition,
// production wiring always passes an Authorizer.
func authorizeGeneration(w http.ResponseWriter, r *http.Request, authorizer iam.AuthorizationPort, action string, projectID int64) bool {
	if authorizer == nil {
		return true
	}
	principal, ok := iam.PrincipalFromContext(r.Context())
	if !ok {
		writeGenerationError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return false
	}
	if projectID <= 0 {
		writeGenerationError(w, http.StatusForbidden, "forbidden", "project scope is required for authorization")
		return false
	}
	if err := authorizer.Authorize(r.Context(), principal, action, strconv.FormatInt(projectID, 10)); err != nil {
		if errors.Is(err, iam.ErrUnauthenticated) || errors.Is(err, iam.ErrInvalidPrincipal) {
			writeGenerationError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return false
		}
		writeGenerationError(w, http.StatusForbidden, "forbidden", "insufficient permission for this project")
		return false
	}
	return true
}

// generationTenant returns the RLS tenant or writes the frozen 400 response.
func generationTenant(w http.ResponseWriter, r *http.Request) (int64, bool) {
	team, ok := rls.TenantFrom(r.Context())
	if !ok || team <= 0 {
		writeGenerationError(w, http.StatusBadRequest, "tenant_required", "tenant header is required")
		return 0, false
	}
	return team, true
}

// generationActor returns the authenticated actor id, or writes 401.
func generationActor(w http.ResponseWriter, r *http.Request) (int64, bool) {
	principal, ok := iam.PrincipalFromContext(r.Context())
	if !ok {
		writeGenerationError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return 0, false
	}
	actor, _ := strconv.ParseInt(principal.Subject, 10, 64)
	if actor <= 0 {
		actor = 1
	}
	return actor, true
}
