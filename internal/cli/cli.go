package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/get-felipe/pg-contract/internal/check"
	"github.com/get-felipe/pg-contract/internal/config"
	"github.com/get-felipe/pg-contract/internal/query"
	"github.com/get-felipe/pg-contract/internal/report"
)

const defaultConfigPath = "pg-contract.yaml"
const devVersion = "0.0.0-dev"

var Version = devVersion

const usage = `pg-contract validates whether existing application SQL still works after a Postgres schema change.

Usage:
  pg-contract check --before-url BEFORE --after-url AFTER --queries queries/
  pg-contract check --before-url BEFORE --after-url AFTER --config pg-contract.yaml
  pg-contract init --queries queries/ --out pg-contract.yaml
  pg-contract version
`

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, usage)
		return 0
	}

	switch args[0] {
	case "version", "--version":
		fmt.Fprintf(stdout, "pg-contract %s\n", resolvedVersion())
		return 0
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "init":
		return runInit(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		fmt.Fprint(stderr, usage)
		return 2
	}
}

func resolvedVersion() string {
	if Version != "" && Version != devVersion {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if version := normalizeBuildVersion(info.Main.Version); version != "" {
			return version
		}
	}
	return Version
}

func normalizeBuildVersion(version string) string {
	switch version {
	case "", "(devel)":
		return ""
	default:
		return strings.TrimPrefix(version, "v")
	}
}

func runCheck(args []string, stdout io.Writer, stderr io.Writer) int {
	var opts check.Options
	var timeout time.Duration
	querySets := stringListFlag{name: "query-set"}
	tags := stringListFlag{name: "tag"}
	noConfig := false
	format := "text"

	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&opts.BeforeURL, "before-url", "", "Postgres URL for the before schema")
	flags.StringVar(&opts.AfterURL, "after-url", "", "Postgres URL for the after schema")
	flags.StringVar(&opts.BeforeURL, "dsn-before", "", "alias for --before-url")
	flags.StringVar(&opts.AfterURL, "dsn-after", "", "alias for --after-url")
	flags.StringVar(&opts.SchemaBefore, "schema-before", "", "optional SQL file to apply to the before database")
	flags.StringVar(&opts.SchemaAfter, "schema-after", "", "optional SQL file to apply to the after database")
	flags.StringVar(&opts.QueriesPath, "queries", "", "directory containing .sql query files; optional with config version 0.2 query_sets")
	flags.StringVar(&opts.ConfigPath, "config", "", "optional pg-contract YAML config file")
	flags.BoolVar(&noConfig, "no-config", noConfig, "do not auto-load pg-contract.yaml")
	flags.Var(&querySets, "query-set", "manifest v0.2 query set to run; may be repeated")
	flags.Var(&tags, "tag", "manifest v0.2 tag to run; may be repeated")
	flags.StringVar(&format, "format", format, "output format: text, json, or github")
	flags.DurationVar(&timeout, "timeout", 30*time.Second, "maximum time for the check")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n", flags.Arg(0))
		return 2
	}
	opts.QuerySets = querySets.Values()
	opts.Tags = tags.Values()
	configWasSet := flagWasSet(flags, "config")
	if noConfig && configWasSet {
		fmt.Fprintln(stderr, "check failed: --config and --no-config cannot be used together")
		return 2
	}
	if configWasSet && opts.ConfigPath == "" {
		fmt.Fprintln(stderr, "check failed: --config cannot be empty")
		return 2
	}
	if !noConfig && !configWasSet {
		configPath, err := detectConfig(defaultConfigPath)
		if err != nil {
			fmt.Fprintf(stderr, "check failed: %v\n", err)
			return 2
		}
		opts.ConfigPath = configPath
	}
	if format != "text" && format != "json" && format != "github" {
		fmt.Fprintf(stderr, "invalid --format %q; expected text, json, or github\n", format)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	result, err := check.Run(ctx, opts)
	if err != nil {
		fmt.Fprintf(stderr, "check failed: %v\n", err)
		return 2
	}

	switch format {
	case "text":
		report.WriteText(stdout, result)
	case "json":
		if err := report.WriteJSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "check failed: %v\n", err)
			return 2
		}
	case "github":
		report.WriteGitHub(stdout, result)
	}
	return check.ExitCode(result)
}

type stringListFlag struct {
	name   string
	values []string
}

func (f *stringListFlag) Set(value string) error {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return fmt.Errorf("%s cannot be empty", f.name)
	}
	f.values = append(f.values, normalized)
	return nil
}

func (f *stringListFlag) String() string {
	if f == nil {
		return ""
	}
	return strings.Join(f.values, ",")
}

func (f *stringListFlag) Values() []string {
	if f == nil {
		return nil
	}
	out := make([]string, len(f.values))
	copy(out, f.values)
	return out
}

func flagWasSet(flags *flag.FlagSet, name string) bool {
	wasSet := false
	flags.Visit(func(flag *flag.Flag) {
		if flag.Name == name {
			wasSet = true
		}
	})
	return wasSet
}

func detectConfig(path string) (string, error) {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return "", fmt.Errorf("auto config path %s is a directory", path)
		}
		return path, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return "", fmt.Errorf("stat auto config path %s: %w", path, err)
}

func runInit(args []string, stdout io.Writer, stderr io.Writer) int {
	queriesPath := ""
	outPath := "pg-contract.yaml"
	force := false

	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&queriesPath, "queries", "", "directory containing .sql query files")
	flags.StringVar(&outPath, "out", outPath, "output config path, or - for stdout")
	flags.BoolVar(&force, "force", force, "overwrite an existing config file")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n", flags.Arg(0))
		return 2
	}
	if queriesPath == "" {
		fmt.Fprintln(stderr, "init failed: missing required --queries")
		return 2
	}
	if outPath == "" {
		fmt.Fprintln(stderr, "init failed: --out cannot be empty")
		return 2
	}

	queries, err := query.LoadDir(queriesPath)
	if err != nil {
		fmt.Fprintf(stderr, "init failed: %v\n", err)
		return 2
	}

	names := make([]string, 0, len(queries))
	for _, q := range queries {
		names = append(names, q.Name)
	}
	data := config.Generate(names)

	if outPath == "-" {
		if _, err := stdout.Write(data); err != nil {
			fmt.Fprintf(stderr, "init failed: write stdout: %v\n", err)
			return 2
		}
		return 0
	}

	flagsValue := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if !force {
		flagsValue = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	}

	file, err := os.OpenFile(outPath, flagsValue, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			fmt.Fprintf(stderr, "init failed: %s already exists; use --force to overwrite\n", outPath)
			return 2
		}
		fmt.Fprintf(stderr, "init failed: open %s: %v\n", outPath, err)
		return 2
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		fmt.Fprintf(stderr, "init failed: write %s: %v\n", outPath, err)
		return 2
	}

	fmt.Fprintf(stdout, "Wrote %s with %d query entries.\n", outPath, len(names))
	return 0
}
