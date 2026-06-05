package contract

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	Version               = "0.1"
	ToolName              = "pg-contract"
	SourceModeLegacy      = "legacy"
	SourceModeManifestV02 = "manifest-v0.2"
)

type Contract struct {
	Version string  `json:"version"`
	Tool    Tool    `json:"tool"`
	Source  Source  `json:"source"`
	Scope   Scope   `json:"scope"`
	Queries []Query `json:"queries"`
}

type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type Source struct {
	Config string `json:"config,omitempty"`
	Mode   string `json:"mode"`
}

type Scope struct {
	Complete  bool     `json:"complete"`
	QuerySets []string `json:"query_sets,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

type Query struct {
	Name        string   `json:"name"`
	QuerySet    string   `json:"query_set,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	File        string   `json:"file"`
	Line        int      `json:"line"`
	SQLSHA256   string   `json:"sql_sha256"`
	Params      []string `json:"params,omitempty"`
	ResultShape []Column `json:"result_shape"`
}

type Column struct {
	Position int    `json:"position"`
	Name     string `json:"name"`
	Type     string `json:"type"`
}

func New(source Source, scope Scope, queries []Query, toolVersion string) Contract {
	return Contract{
		Version: Version,
		Tool: Tool{
			Name:    ToolName,
			Version: strings.TrimSpace(toolVersion),
		},
		Source:  source,
		Scope:   scope,
		Queries: queries,
	}
}

func Load(reader io.Reader) (Contract, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()

	var contract Contract
	if err := decoder.Decode(&contract); err != nil {
		return Contract{}, fmt.Errorf("decode contract: %w", err)
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Contract{}, fmt.Errorf("decode contract: multiple JSON documents are not supported")
		}
		return Contract{}, fmt.Errorf("decode contract: %w", err)
	}

	if err := contract.Validate(); err != nil {
		return Contract{}, err
	}

	return contract, nil
}

func Write(writer io.Writer, contract Contract) error {
	if err := contract.Validate(); err != nil {
		return err
	}

	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(contract); err != nil {
		return fmt.Errorf("write contract: %w", err)
	}
	return nil
}

func (c Contract) Validate() error {
	if c.Version != Version {
		return fmt.Errorf("unsupported contract version %q", c.Version)
	}
	if c.Tool.Name != ToolName {
		return fmt.Errorf("unsupported contract tool %q", c.Tool.Name)
	}
	switch c.Source.Mode {
	case SourceModeLegacy, SourceModeManifestV02:
	default:
		return fmt.Errorf("unsupported contract source mode %q", c.Source.Mode)
	}
	if len(c.Queries) == 0 {
		return fmt.Errorf("contract requires at least one query")
	}

	seen := map[string]struct{}{}
	for i, query := range c.Queries {
		if err := validateQuery(i, query); err != nil {
			return err
		}
		key := queryKey(query)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("contract query %d duplicates query identity %q", i+1, key)
		}
		seen[key] = struct{}{}
	}

	return nil
}

func validateQuery(index int, query Query) error {
	label := fmt.Sprintf("contract query %d", index+1)
	if strings.TrimSpace(query.Name) == "" {
		return fmt.Errorf("%s name cannot be empty", label)
	}
	if strings.TrimSpace(query.File) == "" {
		return fmt.Errorf("%s file cannot be empty", label)
	}
	if query.Line < 1 {
		return fmt.Errorf("%s line must be greater than zero", label)
	}
	hash, ok := strings.CutPrefix(query.SQLSHA256, "sha256:")
	if !ok || len(hash) != 64 {
		return fmt.Errorf("%s sql_sha256 must be a sha256-prefixed hex digest", label)
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return fmt.Errorf("%s sql_sha256 must be a sha256-prefixed hex digest", label)
	}
	if query.ResultShape == nil {
		return fmt.Errorf("%s result_shape cannot be null", label)
	}
	for i, column := range query.ResultShape {
		if err := validateColumn(label, i, column); err != nil {
			return err
		}
	}
	return nil
}

func validateColumn(queryLabel string, index int, column Column) error {
	label := fmt.Sprintf("%s result_shape column %d", queryLabel, index+1)
	if column.Position < 1 {
		return fmt.Errorf("%s position must be greater than zero", label)
	}
	if strings.TrimSpace(column.Name) == "" {
		return fmt.Errorf("%s name cannot be empty", label)
	}
	if strings.TrimSpace(column.Type) == "" {
		return fmt.Errorf("%s type cannot be empty", label)
	}
	return nil
}

func queryKey(query Query) string {
	if query.QuerySet == "" {
		return query.Name
	}
	return query.QuerySet + "/" + query.Name
}
