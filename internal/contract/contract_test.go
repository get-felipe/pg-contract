package contract

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteProducesDeterministicIndentedJSON(t *testing.T) {
	contract := sampleContract()

	var first bytes.Buffer
	if err := Write(&first, contract); err != nil {
		t.Fatalf("write first contract: %v", err)
	}
	var second bytes.Buffer
	if err := Write(&second, contract); err != nil {
		t.Fatalf("write second contract: %v", err)
	}

	if first.String() != second.String() {
		t.Fatalf("expected deterministic JSON\nfirst:\n%s\nsecond:\n%s", first.String(), second.String())
	}
	for _, want := range []string{
		"\"version\": \"0.1\"",
		"\"name\": \"pg-contract\"",
		"\"sql_sha256\": \"sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\"",
		"\"result_shape\": [",
	} {
		if !strings.Contains(first.String(), want) {
			t.Fatalf("expected JSON to contain %q, got:\n%s", want, first.String())
		}
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	input := `{
  "version": "0.1",
  "unknown": true,
  "tool": {"name": "pg-contract"},
  "source": {"mode": "legacy"},
  "scope": {"complete": true},
  "queries": []
}`

	_, err := Load(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestLoadRejectsMultipleDocuments(t *testing.T) {
	input := `{
  "version": "0.1",
  "tool": {"name": "pg-contract"},
  "source": {"mode": "legacy"},
  "scope": {"complete": true},
  "queries": [
    {
      "name": "customers.find",
      "file": "queries/find.sql",
      "line": 2,
      "sql_sha256": "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
      "result_shape": [{"position": 1, "name": "id", "type": "uuid"}]
    }
  ]
}
{}`

	_, err := Load(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "multiple JSON documents") {
		t.Fatalf("expected multiple document error, got %v", err)
	}
}

func TestValidateRejectsUnsupportedVersion(t *testing.T) {
	contract := sampleContract()
	contract.Version = "0.2"

	err := contract.Validate()
	if err == nil || !strings.Contains(err.Error(), "unsupported contract version") {
		t.Fatalf("expected unsupported version error, got %v", err)
	}
}

func TestValidateRejectsUnsupportedSourceMode(t *testing.T) {
	contract := sampleContract()
	contract.Source.Mode = "future"

	err := contract.Validate()
	if err == nil || !strings.Contains(err.Error(), "unsupported contract source mode") {
		t.Fatalf("expected unsupported source mode error, got %v", err)
	}
}

func TestValidateRejectsDuplicateQueryIdentity(t *testing.T) {
	contract := sampleContract()
	contract.Queries = append(contract.Queries, contract.Queries[0])

	err := contract.Validate()
	if err == nil || !strings.Contains(err.Error(), "duplicates query identity") {
		t.Fatalf("expected duplicate query identity error, got %v", err)
	}
}

func TestValidateRejectsInvalidSQLHash(t *testing.T) {
	contract := sampleContract()
	contract.Queries[0].SQLSHA256 = "sha256:zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"

	err := contract.Validate()
	if err == nil || !strings.Contains(err.Error(), "sql_sha256") {
		t.Fatalf("expected invalid hash error, got %v", err)
	}
}

func TestValidateAllowsEmptyResultShape(t *testing.T) {
	contract := sampleContract()
	contract.Queries[0].ResultShape = []Column{}

	if err := contract.Validate(); err != nil {
		t.Fatalf("expected empty result shape to be valid, got %v", err)
	}
}

func TestValidateRejectsNullResultShape(t *testing.T) {
	contract := sampleContract()
	contract.Queries[0].ResultShape = nil

	err := contract.Validate()
	if err == nil || !strings.Contains(err.Error(), "result_shape cannot be null") {
		t.Fatalf("expected null result shape error, got %v", err)
	}
}

func TestLoadRoundTrip(t *testing.T) {
	var data bytes.Buffer
	if err := Write(&data, sampleContract()); err != nil {
		t.Fatalf("write contract: %v", err)
	}

	got, err := Load(&data)
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}

	if got.Version != Version {
		t.Fatalf("expected version %q, got %q", Version, got.Version)
	}
	if got.Queries[0].ResultShape[0].Type != "uuid" {
		t.Fatalf("unexpected result shape: %#v", got.Queries[0].ResultShape)
	}
}

func sampleContract() Contract {
	return New(
		Source{Config: "pg-contract.yaml", Mode: SourceModeManifestV02},
		Scope{Complete: true},
		[]Query{
			{
				Name:      "customers.find",
				QuerySet:  "app",
				Tags:      []string{"customer-facing"},
				File:      "queries/find.sql",
				Line:      2,
				SQLSHA256: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
				Params:    []string{"uuid"},
				ResultShape: []Column{
					{Position: 1, Name: "id", Type: "uuid"},
				},
			},
		},
		"0.1.0-alpha.7",
	)
}
