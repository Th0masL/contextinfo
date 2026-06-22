// Command contextinfo detects the run context (git, repository, CI, runtime) and
// prints it in a choice of formats (envvar, json, text, tfvars).
package main

import (
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
	format := flag.String("format", "envvar", "output format: envvar, json, text, or tfvars")
	prefix := flag.String("prefix", "", "prefix for variable names (applies to envvar, json, tfvars)")
	dir := flag.String("dir", "", "directory to inspect (default: current directory)")
	noFilesChecksum := flag.Bool("no-files-checksum", false, "skip files_checksum (avoids reading every non-ignored file)")
	explain := flag.Bool("explain", false, "also emit <name>_explained companions noting each value's source")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	var opts []contextinfo.Option
	if *dir != "" {
		opts = append(opts, contextinfo.WithDir(*dir))
	}
	if *noFilesChecksum {
		opts = append(opts, contextinfo.WithoutFilesChecksum())
	}
	if *explain {
		opts = append(opts, contextinfo.WithExplain())
	}
	info := contextinfo.Detect(opts...)

	switch *format {
	case "envvar":
		fmt.Print(info.EnvVars(*prefix))
	case "json":
		b, err := info.FlatJSON(*prefix)
		emit(b, err)
	case "text":
		fmt.Print(info.Text())
	case "tfvars":
		fmt.Print(info.TFVarsHCL(*prefix))
	default:
		fmt.Fprintf(os.Stderr, "contextinfo: unknown format %q (want envvar, json, text, or tfvars)\n", *format)
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
	fmt.Fprintf(out, "  envvar  shell NAME=value lines (default)\n")
	fmt.Fprintf(out, "  json    flat JSON object\n")
	fmt.Fprintf(out, "  text    aligned key/value text\n")
	fmt.Fprintf(out, "  tfvars  Terraform variables (HCL)\n")
	fmt.Fprintf(out, "\nExamples:\n")
	fmt.Fprintf(out, "  %s                                    # envvar lines\n", name)
	fmt.Fprintf(out, "  %s --format=json                      # flat JSON\n", name)
	fmt.Fprintf(out, "  %s --explain                          # add <name>_explained source notes\n", name)
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
