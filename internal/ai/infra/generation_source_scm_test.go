package infra

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
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

func newTestGitHubReader(fn roundTripFunc) *githubSourceReader {
	reader := newGitHubSourceReader("token")
	reader.apiRoot = "https://api.github.test"
	reader.client = &http.Client{Transport: fn}
	return reader
}

func TestGitHubSourceValidateRepo(t *testing.T) {
	reader := newTestGitHubReader(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/repos/openware-io/open-im-server":
			return fakeResponse(http.StatusOK, `{"full_name":"openware-io/open-im-server"}`, nil), nil
		case "/repos/openware-io/open-im-server/contents/":
			return fakeResponse(http.StatusOK, `[]`, nil), nil
		default:
			return fakeResponse(http.StatusNotFound, `not found`, nil), nil
		}
	})
	if err := reader.ValidateRepo(context.Background(), "https://github.com/openware-io/open-im-server.git"); err != nil {
		t.Fatalf("validate readable repo: %v", err)
	}
}

func TestGitHubSourceValidateRepoRejectsUnreadableContents(t *testing.T) {
	reader := newTestGitHubReader(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/repos/openware-io/private-repo" {
			return fakeResponse(http.StatusOK, `{"full_name":"openware-io/private-repo"}`, nil), nil
		}
		return fakeResponse(http.StatusNotFound, `not found`, nil), nil
	})
	err := reader.ValidateRepo(context.Background(), "https://github.com/openware-io/private-repo.git")
	if !errors.Is(err, ErrRepositoryNotReadable) {
		t.Fatalf("expected ErrRepositoryNotReadable, got %v", err)
	}
}

func TestGitHubSourceListRefsPaginates(t *testing.T) {
	reader := newTestGitHubReader(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/repos/openware-io/open-im-server/tags" {
			return fakeResponse(http.StatusOK, `[]`, nil), nil
		}
		if r.URL.Path != "/repos/openware-io/open-im-server/branches" {
			return fakeResponse(http.StatusNotFound, `not found`, nil), nil
		}
		page := r.URL.Query().Get("page")
		count := 100
		if page == "2" {
			count = 1
		}
		var items []string
		for i := 0; i < count; i++ {
			items = append(items, fmt.Sprintf(`{"name":"branch-%s-%03d","commit":{"sha":"sha-%s-%03d"}}`, page, i, page, i))
		}
		return fakeResponse(http.StatusOK, "["+strings.Join(items, ",")+"]", nil), nil
	})
	refs, err := reader.ListRefs(context.Background(), "https://github.com/openware-io/open-im-server.git")
	if err != nil {
		t.Fatalf("list refs: %v", err)
	}
	if len(refs) != 101 {
		t.Fatalf("expected 101 branches, got %d", len(refs))
	}
}

func TestGitHubSourceGetRetriesRateLimit(t *testing.T) {
	var calls int32
	reader := newTestGitHubReader(func(r *http.Request) (*http.Response, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			header := http.Header{}
			header.Set("Retry-After", "0")
			return fakeResponse(http.StatusTooManyRequests, `rate limited`, header), nil
		}
		return fakeResponse(http.StatusOK, `{"ok":true}`, nil), nil
	})
	body, err := reader.get(context.Background(), "https://api.github.test/rate-limited", 1024)
	if err != nil || string(body) != `{"ok":true}` {
		t.Fatalf("retry result: body=%q err=%v", body, err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 attempts, got %d", calls)
	}
}
