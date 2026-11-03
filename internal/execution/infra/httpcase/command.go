package httpcase

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// BuildCommand converts a reviewed declarative HTTP case to a fixed curl
// invocation. The host can only come from the operator-configured runtime;
// model output is limited to a relative path, safe method, small JSON body,
// allow-listed headers, and deterministic assertions.
//
//nolint:gocyclo // legacy complexity; refactor tracked in lint backlog
func BuildCommand(script any, baseURL string) ([]string, error) {
	root, ok := script.(map[string]any)
	if !ok {
		return nil, errors.New("generated API case must be a JSON object")
	}
	request, ok := root["request"].(map[string]any)
	if !ok {
		return nil, errors.New("generated API case request is missing")
	}
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("registered environment URL is not a valid origin")
	}
	method, _ := request["method"].(string)
	method = strings.ToUpper(strings.TrimSpace(method))
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
	default:
		return nil, errors.New("generated API case uses a disallowed HTTP method")
	}
	apiPath, _ := request["path"].(string)
	if err := validatePath(apiPath); err != nil {
		return nil, err
	}
	headers, err := readHeaders(request["headers"])
	if err != nil {
		return nil, err
	}
	assertions, err := readAssertions(root["assertions"])
	if err != nil {
		return nil, err
	}
	args := []string{"curl", "--silent", "--show-error", "--max-time", "60", "--request", method}
	for name, value := range headers {
		args = append(args, "--header", name+": "+value)
	}
	if body := request["body"]; body != nil {
		encoded, err := json.Marshal(body)
		if err != nil || len(encoded) > 16*1024 {
			return nil, errors.New("generated API request body is invalid or too large")
		}
		args = append(args, "--data-binary", string(encoded))
	}
	args = append(args, "--write-out", `\n%{http_code}`, strings.TrimRight(baseURL, "/")+apiPath)
	quoted := make([]string, len(args))
	for i := range args {
		quoted[i] = shellQuote(args[i])
	}
	// Keep a non-newline sentinel as the final byte while capturing curl output;
	// otherwise POSIX command substitution strips the response/status separator
	// and turns a valid HTTP response into an empty status.
	scriptBody := "set -eu\nresult=$( " + strings.Join(quoted, " ") + "; printf x)\nresult=${result%x}\nnl=$(printf '\\nx')\nnl=${nl%x}\nstatus=${result##*\"$nl\"}\nbody=${result%\"$nl\"*}\n"
	for _, assertion := range assertions {
		if assertion.kind == "status" {
			expected := strconv.Itoa(assertion.status)
			scriptBody += "[ \"$status\" = " + shellQuote(expected) + " ] || { printf 'HTTP status expected " + expected + ", got %s\\n' \"$status\" >&2; exit 1; }\n"
		} else {
			scriptBody += "printf '%s' \"$body\" | grep -F -- " + shellQuote(assertion.value) + " >/dev/null || { printf 'body assertion failed\\n' >&2; exit 1; }\n"
		}
	}
	scriptBody += "printf 'http_status=%s assertions=passed\\n' \"$status\"\n"
	return []string{"sh", "-c", scriptBody}, nil
}

type assertion struct {
	kind   string
	status int
	value  string
}

func readAssertions(raw any) ([]assertion, error) {
	items, ok := raw.([]any)
	if !ok || len(items) == 0 || len(items) > 8 {
		return nil, errors.New("generated API case assertions are missing or invalid")
	}
	out := make([]assertion, 0, len(items))
	statusCount := 0
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, errors.New("generated API case assertion must be an object")
		}
		kind, _ := row["type"].(string)
		switch kind {
		case "status":
			statusCount++
			status, ok := numberToInt(row["expected_status"])
			if !ok || status < 100 || status > 599 {
				return nil, errors.New("generated API status assertion must be between 100 and 599")
			}
			out = append(out, assertion{kind: kind, status: status})
		case "body_contains":
			value, _ := row["value"].(string)
			if strings.TrimSpace(value) == "" || len(value) > 256 || strings.ContainsAny(value, "\x00") {
				return nil, errors.New("generated API body assertion is invalid")
			}
			out = append(out, assertion{kind: kind, value: value})
		default:
			return nil, fmt.Errorf("generated API assertion %q is not supported", kind)
		}
	}
	if statusCount != 1 {
		return nil, errors.New("generated API case must have exactly one status assertion")
	}
	return out, nil
}

func readHeaders(raw any) (map[string]string, error) {
	if raw == nil {
		return map[string]string{}, nil
	}
	var values map[string]string
	switch headers := raw.(type) {
	case map[string]string:
		values = headers
	case map[string]any:
		values = make(map[string]string, len(headers))
		for key, value := range headers {
			text, ok := value.(string)
			if !ok {
				return nil, errors.New("generated API request header values must be strings")
			}
			values[key] = text
		}
	default:
		return nil, errors.New("generated API request headers must be an object")
	}
	if len(values) > 8 {
		return nil, errors.New("generated API request has too many headers")
	}
	allowed := map[string]string{"accept": "Accept", "content-type": "Content-Type", "x-api-version": "X-API-Version"}
	out := make(map[string]string, len(values))
	for key, value := range values {
		canonical, ok := allowed[strings.ToLower(key)]
		if !ok || len(value) > 512 || strings.ContainsAny(value, "\r\n\x00") {
			return nil, fmt.Errorf("generated API request header %q is not allowed", key)
		}
		out[canonical] = value
	}
	return out, nil
}

func validatePath(value string) error {
	if value == "" || len(value) > 2048 || !strings.HasPrefix(value, "/") || strings.Contains(value, "//") || strings.Contains(value, "..") || strings.ContainsAny(value, "\\\r\n\x00#;$`") {
		return errors.New("generated API request path must be a safe relative URL path")
	}
	u, err := url.Parse(value)
	if err != nil || u.IsAbs() || u.Host != "" || u.User != nil || u.Fragment != "" {
		return errors.New("generated API request path cannot contain a host or credentials")
	}
	return nil
}

func numberToInt(raw any) (int, bool) {
	switch n := raw.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n == float64(int(n)) {
			return int(n), true
		}
	}
	return 0, false
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
