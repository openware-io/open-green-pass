package httperr

import (
	"net/http"
	"testing"

	"github.com/openware-io/open-green-pass/internal/iam"
)

func TestStatusMapsIAMAuthorizationErrors(t *testing.T) {
	if got := Status(iam.ErrUnauthenticated); got != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", got)
	}
	if got := Status(iam.ErrForbidden); got != http.StatusForbidden {
		t.Fatalf("forbidden status=%d", got)
	}
}
