// Command contextinfo detects the run context (CI, git, runtime) and prints it
// in a choice of formats (envvar, json, json-flat, text, tfvars, tfvars-json).
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

func main() {
	format := flag.String("format", "envvar", "output format: envvar, json, json-flat, text, tfvars, or tfvars-json")
	prefix := flag.String("prefix", "", "prefix for variable names (applies to envvar, json-flat, tfvars, tfvars-json)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	info := contextinfo.Detect()

	switch *format {
	case "envvar":
		fmt.Print(info.EnvVars(*prefix))
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(info)
	case "json-flat":
		b, err := info.FlatJSON(*prefix)
		emit(b, err)
	case "text":
		printText(info)
	case "tfvars":
		fmt.Print(info.TFVarsHCL(*prefix))
	case "tfvars-json":
		b, err := info.TFVarsJSON(*prefix)
		emit(b, err)
	default:
		fmt.Fprintf(os.Stderr, "contextinfo: unknown format %q (want envvar, json, json-flat, text, tfvars, or tfvars-json)\n", *format)
		os.Exit(2)
	}
}

// usage prints a description, the flags, the available formats, and examples.
func usage() {
	out := flag.CommandLine.Output()
	name := filepath.Base(os.Args[0])
	fmt.Fprintf(out, "contextinfo — detect CI/CD, git, and runtime context and print it.\n\n")
	fmt.Fprintf(out, "Usage:\n  %s [flags]\n\nFlags:\n", name)
	flag.PrintDefaults()
	fmt.Fprintf(out, "\nFormats:\n")
	fmt.Fprintf(out, "  envvar       shell NAME=value lines (default)\n")
	fmt.Fprintf(out, "  json         nested JSON\n")
	fmt.Fprintf(out, "  json-flat    flat JSON (ci.name -> ci_name)\n")
	fmt.Fprintf(out, "  text         aligned key/value text\n")
	fmt.Fprintf(out, "  tfvars       Terraform variables (HCL)\n")
	fmt.Fprintf(out, "  tfvars-json  Terraform variables (JSON)\n")
	fmt.Fprintf(out, "\nExamples:\n")
	fmt.Fprintf(out, "  %s                                    # envvar lines\n", name)
	fmt.Fprintf(out, "  %s --format=json                      # nested JSON\n", name)
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

func printText(info contextinfo.Info) {
	rows := [][2]string{
		{"ci.detected", fmt.Sprintf("%t", info.CI.Detected)},
		{"ci.name", info.CI.Name},
		{"ci.build_url", info.CI.BuildURL},
		{"ci.build_number", info.CI.BuildNumber},
		{"git.commit", info.Git.Commit},
		{"git.branch", info.Git.Branch},
		{"git.tag", info.Git.Tag},
		{"git.dirty", fmt.Sprintf("%t", info.Git.Dirty)},
		{"git.remote", info.Git.Remote},
		{"runtime.os", info.Runtime.OS},
		{"runtime.arch", info.Runtime.Arch},
		{"runtime.hostname", info.Runtime.Hostname},
	}
	for _, r := range rows {
		fmt.Printf("%-17s %s\n", r[0], r[1])
	}
}
