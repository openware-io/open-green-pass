package infra

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"sigs.k8s.io/yaml"
)

const (
	maxSourceFiles      = 12
	maxSourceFileBytes  = 24 * 1024
	maxSourceTotalBytes = 96 * 1024
	maxGitTreeBytes     = 8 * 1024 * 1024
)

type sourceFile struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	Content string `json:"content"`
}

type sourceTreeResponse struct {
	SHA  string `json:"sha"`
	Tree []struct {
		Path string `json:"path"`
		Type string `json:"type"`
		Size int    `json:"size"`
		SHA  string `json:"sha"`
	} `json:"tree"`
}

type sourceCommitResponse struct {
	SHA    string `json:"sha"`
	Commit struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	} `json:"commit"`
}

type sourceBlobResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

type normalizedSource struct {
	Files      []map[string]string `json:"files"`
	Operations []apiOperation      `json:"operations"`
}

type apiOperation struct {
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	Summary     string   `json:"summary,omitempty"`
	OperationID string   `json:"operation_id,omitempty"`
	Responses   []string `json:"responses,omitempty"`
}

type githubSourceReader struct {
	client  *http.Client
	apiRoot string
	token   string
}

// repositoryRef is deliberately resolved by the server-side SCM connector.
// The browser receives names and immutable commit IDs, never credentials or
// an ability to pass a repository URL to the reader.
type repositoryRef struct {
	Type string
	Name string
	SHA  string
}

func newGitHubSourceReader(token string) *githubSourceReader {
	// The token comes from the repository's bound SCM connection. It is resolved
	// inside server/worker from a Secret reference and never from a browser
	// request, a generation batch, or an activity error.
	return &githubSourceReader{client: &http.Client{Timeout: 15 * time.Second}, apiRoot: "https://api.github.com", token: strings.TrimSpace(token)}
}

func (r *githubSourceReader) Fetch(ctx context.Context, repoURL, branch, sourcePath string) ([]sourceFile, string, error) {
	owner, repo, err := parsePublicGitHubRepository(repoURL)
	if err != nil {
		return nil, "", err
	}
	if !safeBranch(branch) {
		return nil, "", fmt.Errorf("invalid source branch")
	}
	root, err := safeSourceRoot(sourcePath)
	if err != nil {
		return nil, "", err
	}
	commitURL := fmt.Sprintf("%s/repos/%s/%s/commits/%s", r.apiRoot, url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(branch))
	commitBody, err := r.get(ctx, commitURL, 256*1024)
	if err != nil {
		return nil, "", fmt.Errorf("resolve GitHub source commit: %w", err)
	}
	var commit sourceCommitResponse
	if err := json.Unmarshal(commitBody, &commit); err != nil || commit.SHA == "" || commit.Commit.Tree.SHA == "" {
		return nil, "", fmt.Errorf("GitHub source reference did not resolve to a commit and tree")
	}
	treeURL := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s?recursive=1", r.apiRoot, url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(commit.Commit.Tree.SHA))
	body, err := r.get(ctx, treeURL, maxGitTreeBytes)
	if err != nil {
		return nil, "", fmt.Errorf("read GitHub source tree: %w", err)
	}
	var tree sourceTreeResponse
	if err := json.Unmarshal(body, &tree); err != nil {
		return nil, "", fmt.Errorf("decode GitHub source tree: %w", err)
	}
	if tree.SHA == "" {
		return nil, "", fmt.Errorf("GitHub source tree did not include a tree SHA")
	}
	selected := make([]int, 0, len(tree.Tree))
	for i, item := range tree.Tree {
		if item.Type != "blob" || item.SHA == "" || item.Size <= 0 || item.Size > maxSourceFileBytes || !withinSourceRoot(item.Path, root) || !eligibleSourcePath(item.Path) {
			continue
		}
		selected = append(selected, i)
	}
	sort.SliceStable(selected, func(i, j int) bool {
		left, right := tree.Tree[selected[i]].Path, tree.Tree[selected[j]].Path
		lp, rp := sourcePriority(left), sourcePriority(right)
		if lp != rp {
			return lp < rp
		}
		return left < right
	})

	files := make([]sourceFile, 0, maxSourceFiles)
	used := 0
	for _, index := range selected {
		if len(files) == maxSourceFiles || used >= maxSourceTotalBytes {
			break
		}
		item := tree.Tree[index]
		blobURL := fmt.Sprintf("%s/repos/%s/%s/git/blobs/%s", r.apiRoot, url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(item.SHA))
		blobBody, fetchErr := r.get(ctx, blobURL, maxSourceFileBytes*2)
		if fetchErr != nil {
			continue
		}
		var blob sourceBlobResponse
		if json.Unmarshal(blobBody, &blob) != nil || blob.Encoding != "base64" {
			continue
		}
		content, decodeErr := base64.StdEncoding.DecodeString(strings.ReplaceAll(blob.Content, "\n", ""))
		if decodeErr != nil || len(content) == 0 || len(content) > maxSourceFileBytes || !utf8.Valid(content) || used+len(content) > maxSourceTotalBytes {
			continue
		}
		file := sourceFile{Path: item.Path, Kind: sourceKind(item.Path), Content: string(content)}
		files = append(files, file)
		used += len(content)
	}
	if len(files) == 0 {
		return nil, "", fmt.Errorf("no readable source or contract files found under %q", root)
	}
	return files, commit.SHA, nil
}

func (r *githubSourceReader) ListRefs(ctx context.Context, repoURL string) ([]repositoryRef, error) {
	owner, repo, err := parsePublicGitHubRepository(repoURL)
	if err != nil {
		return nil, err
	}
	base := fmt.Sprintf("%s/repos/%s/%s", r.apiRoot, url.PathEscape(owner), url.PathEscape(repo))
	branchesBody, err := r.get(ctx, base+"/branches?per_page=100", 2*1024*1024)
	if err != nil {
		return nil, fmt.Errorf("list GitHub branches: %w", err)
	}
	var branches []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(branchesBody, &branches); err != nil {
		return nil, fmt.Errorf("decode GitHub branches: %w", err)
	}
	tagsBody, err := r.get(ctx, base+"/tags?per_page=100", 2*1024*1024)
	if err != nil {
		return nil, fmt.Errorf("list GitHub tags: %w", err)
	}
	var tags []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(tagsBody, &tags); err != nil {
		return nil, fmt.Errorf("decode GitHub tags: %w", err)
	}
	refs := make([]repositoryRef, 0, len(branches)+len(tags))
	for _, item := range branches {
		if safeBranch(item.Name) && item.Commit.SHA != "" {
			refs = append(refs, repositoryRef{Type: "branch", Name: item.Name, SHA: item.Commit.SHA})
		}
	}
	for _, item := range tags {
		if safeBranch(item.Name) && item.Commit.SHA != "" {
			commit, resolveErr := r.resolveTagCommit(ctx, owner, repo, item.Commit.SHA)
			if resolveErr != nil {
				return nil, fmt.Errorf("resolve GitHub tag %q: %w", item.Name, resolveErr)
			}
			refs = append(refs, repositoryRef{Type: "tag", Name: item.Name, SHA: commit})
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Type != refs[j].Type {
			return refs[i].Type < refs[j].Type
		}
		return refs[i].Name < refs[j].Name
	})
	return refs, nil
}

func (r *githubSourceReader) resolveTagCommit(ctx context.Context, owner, repo, objectSHA string) (string, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/git/tags/%s", r.apiRoot, url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(objectSHA))
	body, err := r.get(ctx, endpoint, 128*1024)
	if err != nil {
		// GitHub's tag list points directly at a commit for lightweight tags;
		// GET git/tags then returns 404. In that case the listed SHA is already
		// the immutable commit that must be shown to the user.
		if strings.Contains(err.Error(), "HTTP 404") {
			return objectSHA, nil
		}
		return "", err
	}
	var tag struct {
		Object struct {
			SHA  string `json:"sha"`
			Type string `json:"type"`
		} `json:"object"`
	}
	if err := json.Unmarshal(body, &tag); err != nil || tag.Object.SHA == "" {
		return "", fmt.Errorf("decode annotated tag")
	}
	if tag.Object.Type != "commit" {
		return "", fmt.Errorf("annotated tag does not resolve directly to a commit")
	}
	return tag.Object.SHA, nil
}

func (r *githubSourceReader) get(ctx context.Context, endpoint string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "greenpass-generation-worker")
	if r.token != "" {
		req.Header.Set("Authorization", "Bearer "+r.token)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("GitHub response exceeded the configured size limit")
	}
	return body, nil
}

func parsePublicGitHubRepository(raw string) (string, string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Host, "github.com") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", "", fmt.Errorf("source reader only supports public HTTPS GitHub repositories")
	}
	parts := strings.Split(strings.Trim(strings.TrimSuffix(u.Path, ".git"), "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("repository URL must identify one GitHub owner and repository")
	}
	return parts[0], parts[1], nil
}

func safeBranch(branch string) bool {
	if strings.TrimSpace(branch) == "" || len(branch) > 255 || strings.ContainsAny(branch, "\\\x00\r\n?#") || strings.HasPrefix(branch, "-") {
		return false
	}
	for _, part := range strings.Split(branch, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func safeSourceRoot(sourcePath string) (string, error) {
	value := strings.TrimSpace(strings.ReplaceAll(sourcePath, "\\", "/"))
	if value == "" || value == "." || value == "/" {
		return "", nil
	}
	if strings.HasPrefix(value, "/") || strings.ContainsRune(value, '\x00') {
		return "", fmt.Errorf("source directory must be a repository-relative path")
	}
	clean := path.Clean(value)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("source directory escapes the repository root")
	}
	return clean, nil
}

func withinSourceRoot(file, root string) bool {
	clean := path.Clean(strings.ReplaceAll(file, "\\", "/"))
	if clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return false
	}
	return root == "" || clean == root || strings.HasPrefix(clean, root+"/")
}

func eligibleSourcePath(file string) bool {
	lower := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	for _, segment := range strings.Split(lower, "/") {
		switch segment {
		case ".git", "node_modules", "vendor", "dist", "build", "coverage", ".next", "target":
			return false
		}
	}
	base := path.Base(lower)
	for _, secret := range []string{".env", "secret", "credential", "private", ".pem", ".key"} {
		if strings.Contains(base, secret) {
			return false
		}
	}
	if strings.HasSuffix(base, ".lock") || strings.HasSuffix(base, ".map") || strings.HasSuffix(base, ".min.js") || strings.HasSuffix(base, ".snap") {
		return false
	}
	switch path.Ext(base) {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".java", ".kt", ".cs", ".rb", ".php", ".json", ".yaml", ".yml", ".toml", ".xml", ".md", ".proto":
		return true
	default:
		return false
	}
}

func sourcePriority(file string) int {
	lower := strings.ToLower(file)
	base := path.Base(lower)
	if strings.Contains(lower, "openapi") || strings.Contains(lower, "swagger") || strings.Contains(lower, "/api/") || strings.Contains(lower, "/routes/") {
		return 0
	}
	if strings.Contains(lower, "test") || strings.Contains(lower, "spec") || base == "readme.md" {
		return 1
	}
	if base == "package.json" || base == "go.mod" || base == "pyproject.toml" || base == "pom.xml" {
		return 2
	}
	return 3
}

func sourceKind(file string) string {
	base := strings.ToLower(path.Base(file))
	ext := path.Ext(base)
	// Generated client/server source and framework configuration often contain
	// "OpenAPI" in their names. Only declarative documents can be contracts;
	// passing Java/Go/TypeScript source into the YAML parser aborts the batch.
	if (ext == ".json" || ext == ".yaml" || ext == ".yml") && (strings.Contains(strings.ToLower(file), "openapi") || strings.Contains(strings.ToLower(file), "swagger")) {
		return "api_contract"
	}
	if strings.Contains(base, "test") || strings.Contains(base, "spec") {
		return "test"
	}
	switch path.Ext(base) {
	case ".json", ".yaml", ".yml", ".toml", ".xml":
		return "configuration"
	case ".md":
		return "documentation"
	default:
		return "source"
	}
}

func normalizeSource(files []sourceFile) (normalizedSource, error) {
	out := normalizedSource{Files: make([]map[string]string, 0, len(files)), Operations: []apiOperation{}}
	for _, file := range files {
		out.Files = append(out.Files, map[string]string{"path": file.Path, "kind": file.Kind})
		if file.Kind != "api_contract" {
			continue
		}
		jsonBytes, err := yaml.YAMLToJSON([]byte(file.Content))
		if err != nil {
			return normalizedSource{}, fmt.Errorf("parse API contract %s: %w", file.Path, err)
		}
		var document map[string]any
		if err := json.Unmarshal(jsonBytes, &document); err != nil {
			return normalizedSource{}, fmt.Errorf("decode API contract %s: %w", file.Path, err)
		}
		// A document in an openapi/swagger directory may be a generator config
		// or a reusable schema, rather than an OpenAPI root document.
		openapi, _ := document["openapi"].(string)
		swagger, _ := document["swagger"].(string)
		if openapi == "" && swagger == "" {
			continue
		}
		paths, _ := document["paths"].(map[string]any)
		pathNames := make([]string, 0, len(paths))
		for name := range paths {
			pathNames = append(pathNames, name)
		}
		sort.Strings(pathNames)
		for _, route := range pathNames {
			methods, _ := paths[route].(map[string]any)
			methodNames := make([]string, 0, len(methods))
			for method := range methods {
				if isHTTPMethod(method) {
					methodNames = append(methodNames, method)
				}
			}
			sort.Strings(methodNames)
			for _, method := range methodNames {
				operation, _ := methods[method].(map[string]any)
				item := apiOperation{Method: strings.ToUpper(method), Path: route}
				item.Summary, _ = operation["summary"].(string)
				item.OperationID, _ = operation["operationId"].(string)
				if responses, ok := operation["responses"].(map[string]any); ok {
					for status := range responses {
						item.Responses = append(item.Responses, status)
					}
					sort.Strings(item.Responses)
				}
				out.Operations = append(out.Operations, item)
			}
		}
	}
	if len(out.Files) == 0 {
		return normalizedSource{}, fmt.Errorf("no source files were available for normalization")
	}
	return out, nil
}

func isHTTPMethod(method string) bool {
	switch strings.ToLower(method) {
	case "get", "post", "put", "patch", "delete", "head", "options":
		return true
	default:
		return false
	}
}

func sourceDigest(files []sourceFile) (string, []byte, error) {
	content, err := json.Marshal(files)
	if err != nil {
		return "", nil, err
	}
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:]), content, nil
}
