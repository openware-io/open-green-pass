package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/iam"
	iamapp "github.com/openware-io/open-green-pass/internal/iam/application"
	iaminfra "github.com/openware-io/open-green-pass/internal/iam/infra"
)

type memberUpdate struct {
	Role   iam.Role         `json:"role"`
	Status iam.MemberStatus `json:"status"`
}
type grantUpdate struct {
	Permission      iam.Permission `json:"permission"`
	ConcurrentQuota int64          `json:"concurrent_quota"`
	SandboxQuota    int64          `json:"sandbox_quota"`
}

func RegisterTeams(mux *http.ServeMux, store *iaminfra.TeamStore, rbac *iaminfra.RBACStore, members *iamapp.MembershipService) {
	mux.HandleFunc("GET /teams/current", func(w http.ResponseWriter, r *http.Request) {
		teamID, _, ok := requestIdentity(w, r)
		if !ok {
			return
		}
		value, err := store.Summary(r.Context(), teamID)
		writeIAM(w, value, err)
	})
	mux.HandleFunc("GET /teams/current/members", func(w http.ResponseWriter, r *http.Request) {
		teamID, _, ok := requestIdentity(w, r)
		if !ok {
			return
		}
		value, err := store.Members(r.Context(), teamID)
		writeIAM(w, value, err)
	})
	mux.HandleFunc("PATCH /teams/current/members/{id}", func(w http.ResponseWriter, r *http.Request) {
		teamID, _, ok := mutationIdentity(w, r)
		if !ok {
			return
		}
		memberID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_member_id"})
			return
		}
		var input memberUpdate
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_request"})
			return
		}
		if input.Status == "" {
			input.Status = iam.MemberActive
		}
		if err = members.ChangeRole(r.Context(), teamID, memberID, input.Role, input.Status); err != nil {
			writeIAM(w, nil, err)
			return
		}
		writeIAM(w, map[string]string{"status": "updated"}, nil)
	})
	mux.HandleFunc("GET /teams/current/assets", func(w http.ResponseWriter, r *http.Request) {
		teamID, _, ok := requestIdentity(w, r)
		if !ok {
			return
		}
		value, err := store.Assets(r.Context(), teamID)
		writeIAM(w, value, err)
	})
	mux.HandleFunc("GET /teams/current/assets/{targetId}/permissions", func(w http.ResponseWriter, r *http.Request) {
		teamID, _, ok := requestIdentity(w, r)
		if !ok {
			return
		}
		targetID, err := strconv.ParseInt(r.PathValue("targetId"), 10, 64)
		if err != nil {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_target_id"})
			return
		}
		value, err := store.AssetGrants(r.Context(), teamID, targetID)
		writeIAM(w, value, err)
	})
	mux.HandleFunc("PUT /teams/current/assets/{targetId}/permissions/{memberId}", func(w http.ResponseWriter, r *http.Request) {
		teamID, actorID, ok := mutationIdentity(w, r)
		if !ok {
			return
		}
		targetID, e1 := strconv.ParseInt(r.PathValue("targetId"), 10, 64)
		memberID, e2 := strconv.ParseInt(r.PathValue("memberId"), 10, 64)
		if e1 != nil || e2 != nil {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_path"})
			return
		}
		var input grantUpdate
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil {
			httpx.WriteJSON(w, 400, map[string]string{"error": "invalid_request"})
			return
		}
		grant := &iam.AssetPermission{TeamID: teamID, TargetID: targetID, MemberID: memberID, Permission: input.Permission, ConcurrentQuota: input.ConcurrentQuota, SandboxQuota: input.SandboxQuota, CreatedBy: actorID, UpdatedBy: actorID}
		if input.Permission == iam.PermissionNone {
			err := rbac.DeleteAssetPermission(r.Context(), teamID, targetID, memberID)
			writeIAM(w, map[string]string{"status": "updated"}, err)
			return
		}
		err := rbac.SaveAssetPermission(r.Context(), grant)
		writeIAM(w, grant, err)
	})
}

func requestIdentity(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	p, ok := iam.PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteJSON(w, 401, map[string]string{"error": "unauthorized"})
		return 0, 0, false
	}
	teamID, e1 := strconv.ParseInt(p.TenantID, 10, 64)
	actorID, e2 := strconv.ParseInt(p.Subject, 10, 64)
	if e1 != nil || e2 != nil {
		httpx.WriteJSON(w, 401, map[string]string{"error": "invalid_identity"})
		return 0, 0, false
	}
	return teamID, actorID, true
}
func mutationIdentity(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	teamID, actorID, ok := requestIdentity(w, r)
	if !ok {
		return 0, 0, false
	}
	p, _ := iam.PrincipalFromContext(r.Context())
	for _, role := range p.Roles {
		if role == string(iam.RoleOwner) || role == string(iam.RoleAdmin) {
			return teamID, actorID, true
		}
	}
	httpx.WriteJSON(w, 403, map[string]string{"error": "owner_or_admin_required"})
	return 0, 0, false
}
func writeIAM(w http.ResponseWriter, value any, err error) {
	if err == nil {
		httpx.WriteJSON(w, 200, value)
		return
	}
	status := 500
	code := "internal_error"
	if errors.Is(err, pgx.ErrNoRows) {
		status = 404
		code = "not_found"
	} else if errors.Is(err, iamapp.ErrLastOwner) {
		status = 409
		code = "last_owner"
	} else if strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "quota") {
		status = 400
		code = "invalid_request"
	}
	httpx.WriteJSON(w, status, map[string]string{"error": code, "message": err.Error()})
}
