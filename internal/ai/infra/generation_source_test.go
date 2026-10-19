package infra

import "testing"

func TestSourceKindDoesNotTreatGeneratedCodeAsContract(t *testing.T) {
	for _, file := range []string{
		"src/main/java/example/config/OpenApiConfiguration.java",
		"docs/swagger.go",
		"src/openapi/client.ts",
	} {
		if got := sourceKind(file); got != "source" {
			t.Errorf("sourceKind(%q) = %q, want source", file, got)
		}
	}
	for _, file := range []string{"openapi.yaml", "contracts/swagger.json", "api/openapi.yml"} {
		if got := sourceKind(file); got != "api_contract" {
			t.Errorf("sourceKind(%q) = %q, want api_contract", file, got)
		}
	}
}

func TestNormalizeSourceHandlesMixedContractsAndFrameworkCode(t *testing.T) {
	files := []sourceFile{
		{Path: "src/OpenApiConfiguration.java", Kind: sourceKind("src/OpenApiConfiguration.java"), Content: "package example; public class OpenApiConfiguration {}"},
		{Path: "openapi/generator.json", Kind: sourceKind("openapi/generator.json"), Content: `{"generatorName":"typescript-fetch","paths":{"output":"generated"}}`},
		{Path: "openapi.yaml", Kind: sourceKind("openapi.yaml"), Content: "openapi: 3.0.3\npaths:\n  /health:\n    get:\n      operationId: health\n      responses:\n        '200':\n          description: OK\n"},
	}
	got, err := normalizeSource(files)
	if err != nil {
		t.Fatalf("normalize mixed source: %v", err)
	}
	if len(got.Files) != 3 || len(got.Operations) != 1 {
		t.Fatalf("expected all source files and one real operation, got %#v", got)
	}
	operation := got.Operations[0]
	if operation.Method != "GET" || operation.Path != "/health" || operation.OperationID != "health" || len(operation.Responses) != 1 || operation.Responses[0] != "200" {
		t.Fatalf("unexpected normalized operation: %#v", operation)
	}
}

func TestNormalizeSourceRejectsMalformedContract(t *testing.T) {
	_, err := normalizeSource([]sourceFile{{Path: "openapi.yaml", Kind: "api_contract", Content: "openapi: [unterminated"}})
	if err == nil {
		t.Fatal("malformed API contract must not be treated as valid evidence")
	}
}
