// Command auditverify validates an exported aud_event chain without writer
// credentials. It intentionally reads a JSON export rather than sharing the
// server's database connection or trusted-writer role.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	trustedapp "github.com/openware-io/open-green-pass/internal/trusted/application"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
)

type auditExport struct {
	Events []domain.AuditEvent `json:"events"`
	Anchor *domain.AuditAnchor `json:"anchor,omitempty"`
}

func main() {
	input := flag.String("input", "", "path to a JSON audit export; use - for stdin")
	maxEvents := flag.Int("max-events", 100000, "maximum events accepted from one export")
	flag.Parse()
	if *input == "" {
		fatal(2, "--input is required")
	}

	reader, closeReader, err := openInput(*input)
	if err != nil {
		fatal(2, "open input: %v", err)
	}
	defer closeReader()

	var export auditExport
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&export); err != nil {
		fatal(2, "decode audit export: %v", err)
	}
	if err := rejectTrailingJSON(decoder); err != nil {
		fatal(2, "decode audit export: %v", err)
	}

	verifier := trustedapp.AuditVerifier{MaxEvents: *maxEvents}
	result, verifyErr := verifier.Verify(context.Background(), export.Events, export.Anchor)
	if verifyErr != nil {
		fatal(2, "verify audit export: %v", verifyErr)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fatal(2, "write result: %v", err)
	}
	if !result.Valid {
		os.Exit(1)
	}
}

func openInput(path string) (io.Reader, func(), error) {
	if path == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, func() {}, err
	}
	return f, func() { _ = f.Close() }, nil
}

func rejectTrailingJSON(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("multiple JSON values are not allowed")
	}
	return err
}

func fatal(code int, format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(code)
}
