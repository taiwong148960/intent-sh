package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/taiwong148960/intent-sh/internal/citest"
)

const maxManifestBytes = 1 << 20

type matrixFlags map[string]string

func (values matrixFlags) String() string {
	return "bounded non-secret key=value metadata"
}

func (values matrixFlags) Set(raw string) error {
	key, value, ok := strings.Cut(raw, "=")
	if !ok || key == "" || value == "" {
		return fmt.Errorf("matrix metadata must be key=value")
	}
	if _, exists := values[key]; exists {
		return fmt.Errorf("matrix metadata key is duplicated")
	}
	values[key] = value
	return nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("test-audit", flag.ContinueOnError)
	flags.SetOutput(stderr)
	manifestPath := flags.String("manifest", ".github/ci/test-manifest.json", "path to the CI test manifest")
	suiteName := flags.String("suite", "", "manifest suite to audit")
	outputPath := flags.String("output", "", "optional path for the sanitized JSON summary")
	maxBytes := flags.Int64("max-bytes", citest.DefaultMaxBytes, "maximum go test JSON input size")
	maxEvents := flags.Int("max-events", citest.DefaultMaxEvents, "maximum go test JSON event count")
	matrix := matrixFlags{}
	flags.Var(matrix, "matrix", "bounded non-secret matrix metadata in key=value form (repeatable)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *suiteName == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: test-audit -suite NAME [-manifest PATH] [-output PATH] [-matrix key=value]")
		return 2
	}

	manifestFile, err := os.Open(*manifestPath)
	if err != nil {
		fmt.Fprintln(stderr, "test manifest could not be opened")
		return 2
	}
	manifest, loadErr := citest.LoadManifest(manifestFile, maxManifestBytes)
	closeErr := manifestFile.Close()
	if loadErr != nil || closeErr != nil {
		fmt.Fprintln(stderr, "test manifest is invalid or unreadable")
		return 2
	}

	summary, auditErr := citest.Audit(manifest, *suiteName, stdin, *maxBytes, *maxEvents)
	if matrixErr := citest.SetMatrix(&summary, matrix); matrixErr != nil {
		fmt.Fprintln(stderr, "matrix metadata is invalid")
		return 2
	}
	if err := writeJSON(stdout, summary); err != nil {
		fmt.Fprintln(stderr, "test summary could not be written")
		return 2
	}
	if *outputPath != "" {
		if err := citest.WriteSummaryFile(*outputPath, summary); err != nil {
			fmt.Fprintln(stderr, "test summary artifact could not be written")
			return 2
		}
	}
	if auditErr != nil {
		fmt.Fprintln(stderr, "test qualification failed; see the bounded summary")
		return 1
	}
	return 0
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(true)
	return encoder.Encode(value)
}
