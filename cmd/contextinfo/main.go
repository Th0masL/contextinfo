// Command contextinfo detects the run context (git, repository, CI, runtime) and
// prints it in a choice of formats (envvar, json, text, tfvars, tfvars-json).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Th0masL/contextinfo/pkg/contextinfo"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

// main parses flags, runs detection, and prints the result in the chosen format.
func main() {
	format := flag.String("format", "envvar", "output format: envvar, json, text, tfvars, or tfvars-json")
	prefix := flag.String("prefix", "", "prefix for variable names (applies to envvar, tfvars, tfvars-json)")
	noChecksum := flag.Bool("no-checksum", false, "skip git_checksum (avoids reading every non-ignored file)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	var opts []contextinfo.Option
	if *noChecksum {
		opts = append(opts, contextinfo.WithoutChecksum())
	}
	info := contextinfo.Detect(opts...)

	switch *format {
	case "envvar":
		fmt.Print(info.EnvVars(*prefix))
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(info)
	case "text":
		printText(info)
	case "tfvars":
		fmt.Print(info.TFVarsHCL(*prefix))
	case "tfvars-json":
		b, err := info.TFVarsJSON(*prefix)
		emit(b, err)
	default:
		fmt.Fprintf(os.Stderr, "contextinfo: unknown format %q (want envvar, json, text, tfvars, or tfvars-json)\n", *format)
		os.Exit(2)
	}
}

// usage prints a description, the flags, the available formats, and examples.
func usage() {
	out := flag.CommandLine.Output()
	name := filepath.Base(os.Args[0])
	fmt.Fprintf(out, "contextinfo — detect git, repository, CI/CD, and runtime context and print it.\n\n")
	fmt.Fprintf(out, "Usage:\n  %s [flags]\n\nFlags:\n", name)
	flag.PrintDefaults()
	fmt.Fprintf(out, "\nFormats:\n")
	fmt.Fprintf(out, "  envvar       shell NAME=value lines (default)\n")
	fmt.Fprintf(out, "  json         flat JSON object\n")
	fmt.Fprintf(out, "  text         aligned key/value text\n")
	fmt.Fprintf(out, "  tfvars       Terraform variables (HCL)\n")
	fmt.Fprintf(out, "  tfvars-json  Terraform variables (JSON)\n")
	fmt.Fprintf(out, "\nExamples:\n")
	fmt.Fprintf(out, "  %s                                    # envvar lines\n", name)
	fmt.Fprintf(out, "  %s --format=json                      # flat JSON\n", name)
	fmt.Fprintf(out, "  %s --format=tfvars > ctx.auto.tfvars  # Terraform vars file\n", name)
	fmt.Fprintf(out, "  set -a; eval \"$(%s --format=envvar --prefix TF_VAR_)\"; set +a   # export TF_VAR_* for terraform\n", name)
}

// emit writes b to stdout, or exits non-zero on a rendering error.
func emit(b []byte, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "contextinfo: %v\n", err)
		os.Exit(1)
	}
	os.Stdout.Write(b)
}

// printText prints the context as aligned "key value" lines, one field per row.
func printText(info contextinfo.Info) {
	rows := [][2]string{
		{"git_branch", info.GitBranch},
		{"git_commit_sha", info.GitCommitSHA},
		{"git_commit_sha_short", info.GitCommitSHAShort},
		{"git_tag", info.GitTag},
		{"git_dirty", fmt.Sprintf("%t", info.GitDirty)},
		{"git_checksum", info.GitChecksum},
		{"git_repo_url", info.GitRepoURL},
		{"git_repository", info.GitRepository},
		{"actor", info.Actor},
		{"event", info.Event},
		{"ci_platform", info.CIPlatform},
		{"ci_build_url", info.CIBuildURL},
		{"ci_build_number", info.CIBuildNumber},
		{"ci_workflow", info.CIWorkflow},
		{"runtime_hostname", info.RuntimeHostname},
	}
	for _, r := range rows {
		fmt.Printf("%-20s %s\n", r[0], r[1])
	}
}
