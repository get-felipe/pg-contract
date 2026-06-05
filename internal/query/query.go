package query

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var namePattern = regexp.MustCompile(`(?i)^\s*--\s*name:\s*([A-Za-z][A-Za-z0-9_.-]*)(?:\s+:[A-Za-z][A-Za-z0-9_-]*)?\s*$`)

type Query struct {
	Name      string
	File      string
	SQL       string
	StartLine int
}

func SQLSHA256(sql string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(sql)))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func LoadDir(root string) ([]Query, error) {
	return LoadPaths([]string{root})
}

func LoadPaths(paths []string) ([]Query, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("queries path is required")
	}

	var files []queryFile
	for _, path := range paths {
		collected, err := collectFiles(path)
		if err != nil {
			return nil, err
		}
		files = append(files, collected...)
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].path == files[j].path {
			return files[i].root < files[j].root
		}
		return files[i].path < files[j].path
	})

	queries := make([]Query, 0, len(files))
	seen := map[string]string{}
	for _, file := range files {
		q, err := loadFile(file.root, file.path)
		if err != nil {
			return nil, err
		}
		if previous, exists := seen[q.Name]; exists {
			return nil, fmt.Errorf("duplicate query name %q in %s and %s", q.Name, previous, q.File)
		}
		seen[q.Name] = q.File
		queries = append(queries, q)
	}

	if len(queries) == 0 {
		return nil, fmt.Errorf("no .sql query files found in %s", strings.Join(paths, ", "))
	}

	return queries, nil
}

type queryFile struct {
	root string
	path string
}

func collectFiles(root string) ([]queryFile, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("queries path is required")
	}

	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat queries path: %w", err)
	}
	if !info.IsDir() {
		if !strings.EqualFold(filepath.Ext(root), ".sql") {
			return nil, fmt.Errorf("queries path must be a directory or .sql file: %s", root)
		}
		return []queryFile{{root: filepath.Dir(root), path: root}}, nil
	}

	var files []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".sql") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk queries path: %w", err)
	}

	sort.Strings(files)
	out := make([]queryFile, 0, len(files))
	for _, file := range files {
		out = append(out, queryFile{root: root, path: file})
	}
	return out, nil
}

func loadFile(root string, file string) (Query, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return Query{}, fmt.Errorf("read query file %s: %w", file, err)
	}

	sql := strings.TrimSpace(string(data))
	if sql == "" {
		return Query{}, fmt.Errorf("query file is empty: %s", file)
	}

	name := ""
	startLine := 1
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if name == "" {
			if matches := namePattern.FindStringSubmatch(line); matches != nil {
				name = matches[1]
			}
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, "--") {
			startLine = i + 1
			break
		}
	}

	if name == "" {
		name = fallbackName(root, file)
	}

	return Query{
		Name:      name,
		File:      file,
		SQL:       sql,
		StartLine: startLine,
	}, nil
}

func fallbackName(root string, file string) string {
	rel, err := filepath.Rel(root, file)
	if err != nil {
		rel = filepath.Base(file)
	}
	rel = filepath.ToSlash(rel)
	rel = strings.TrimSuffix(rel, filepath.Ext(rel))
	rel = strings.Trim(rel, "/")
	rel = strings.ReplaceAll(rel, "/", ".")
	rel = strings.ReplaceAll(rel, " ", "_")
	if rel == "" {
		return "query"
	}
	return rel
}
