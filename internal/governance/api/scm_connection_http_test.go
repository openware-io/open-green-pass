package api

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fakeResponse(status int, body string, header http.Header) *http.Response {
	if header == nil {
		header = http.Header{}
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func withGitHubTransport(t *testing.T, fn roundTripFunc) {
	t.Helper()
	oldBase, oldClient := githubAPIBase, githubHTTPClient
	githubAPIBase = "https://api.github.test"
	githubHTTPClient = &http.Client{Transport: fn}
	t.Cleanup(func() {
		githubAPIBase = oldBase
		githubHTTPClient = oldClient
	})
}

func TestActorIDFromSubject(t *testing.T) {
	if got := actorIDFromSubject("42"); got != 42 {
		t.Fatalf("numeric subject: got %d", got)
	}
	first := actorIDFromSubject("oidc|user@example.com")
	second := actorIDFromSubject("oidc|user@example.com")
	if first <= 0 || first != second {
		t.Fatalf("non-numeric subject must be stable and positive: %d %d", first, second)
	}
	if got := actorIDFromSubject(""); got != 0 {
		t.Fatalf("empty subject: got %d", got)
	}
}

func TestParseKubernetesSecretRef(t *testing.T) {
	ns, name, key, ok := parseKubernetesSecretRef("k8s://gp/gp-scm-1-2#token")
	if !ok || ns != "gp" || name != "gp-scm-1-2" || key != "token" {
		t.Fatalf("unexpected parse: %q %q %q %v", ns, name, key, ok)
	}
	for _, bad := range []string{"", "vault://x", "k8s://gp", "k8s://gp/name", "k8s:///name#token"} {
		if _, _, _, ok := parseKubernetesSecretRef(bad); ok {
			t.Fatalf("expected invalid ref: %q", bad)
		}
	}
}

func TestProbeGitHubReadTokenRejectsClassicScopes(t *testing.T) {
	withGitHubTransport(t, func(_ *http.Request) (*http.Response, error) {
		header := http.Header{}
		header.Set("X-OAuth-Scopes", "repo, workflow")
		return fakeResponse(http.StatusOK, `{"login":"alice"}`, header), nil
	})
	if _, err := probeGitHubReadToken(context.Background(), "classic", "pat"); err == nil || !strings.Contains(err.Error(), "classic token") {
		t.Fatalf("expected classic scope rejection, got %v", err)
	}
}

func TestProbeGitHubReadTokenAllowsFineGrained(t *testing.T) {
	withGitHubTransport(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/user" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return fakeResponse(http.StatusOK, `{"login":"alice"}`, nil), nil
	})
	info, err := probeGitHubReadToken(context.Background(), "fine-grained", "pat")
	if err != nil || info.Login != "alice" || len(info.Scopes) != 0 {
		t.Fatalf("fine-grained token: info=%+v err=%v", info, err)
	}
}

func TestProbeGitHubReadTokenUsesInstallationEndpoint(t *testing.T) {
	withGitHubTransport(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/installation/repositories" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return fakeResponse(http.StatusOK, `{"total_count":0,"repositories":[]}`, nil), nil
	})
	if _, err := probeGitHubReadToken(context.Background(), "installation-token", "github_app"); err != nil {
		t.Fatalf("installation probe: %v", err)
	}
}

func TestGitHubAppConfiguredRequiresFrontendURL(t *testing.T) {
	cfg := GitHubAppConfig{
		AppID: "1", ClientID: "Iv1.x", ClientSecret: "secret", InstallURL: "https://github.com/apps/x/installations/new",
		PrivateKeyRef: "k8s://gp/gp-github-app#private-key", OAuthCallbackURL: "https://gp.example/api/scm-connections/github-app/oauth-callback",
	}
	if cfg.configured() {
		t.Fatal("frontend URL must be required")
	}
	cfg.FrontendURL = "https://gp.example"
	if !cfg.configured() {
		t.Fatal("complete configuration must be accepted")
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if !isUniqueViolation(&pgconn.PgError{Code: "23505"}) {
		t.Fatal("expected unique violation")
	}
	if isUniqueViolation(&pgconn.PgError{Code: "23503"}) {
		t.Fatal("foreign key violation must not be reported as a name conflict")
	}
}

func TestSplitScopes(t *testing.T) {
	got := splitScopes("repo, workflow  gist\nread:org")
	want := []string{"repo", "workflow", "gist", "read:org"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}
