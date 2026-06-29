// Command contextinfo detects the run context (git, repository, CI, runtime) and
// prints it in a choice of formats (envvar, json, text, tfvars).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Th0masL/contextinfo"
	"github.com/Th0masL/contextinfo/config"
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
	envName := flag.String("env-name", "", "force the env_name deploy variable (overrides config deploy rules)")
	buildType := flag.String("build-type", "", "force the build_type deploy variable (overrides config deploy rules)")
	noConfigCascade := flag.Bool("no-config-cascade", false, "read only the closest .contextinfo.yaml (don't merge parent/$HOME/etc configs)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	// Resolve the directory to inspect; it also roots config discovery.
	workdir := *dir
	if workdir == "" {
		if wd, err := os.Getwd(); err == nil {
			workdir = wd
		}
	}

	// Load .contextinfo.yaml (merged closest-wins; --no-config-cascade reads only
	// the closest), then let explicitly-set flags override it. Precedence:
	// defaults < config file(s) < flags.
	var loadOpts []config.LoadOption
	if *noConfigCascade {
		loadOpts = append(loadOpts, config.NoCascade())
	}
	cfg, _, err := config.Load(workdir, loadOpts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "contextinfo: %v\n", err)
		os.Exit(2)
	}
	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })

	outFormat := resolveStr("envvar", cfg.Format, set["format"], *format)
	outPrefix := resolveStr("", cfg.Prefix, set["prefix"], *prefix)
	checksum := resolveBool(true, cfg.FilesChecksum, set["no-files-checksum"], !*noFilesChecksum)
	doExplain := resolveBool(false, cfg.Explain, set["explain"], *explain)

	opts := []contextinfo.Option{contextinfo.WithDir(workdir)}
	if !checksum {
		opts = append(opts, contextinfo.WithoutFilesChecksum())
	}
	// Deploy rules come from the config file; --env-name/--build-type force a
	// variable directly, overriding whatever the rules would set.
	if rules, ok := cfg.DeployRules(); ok {
		opts = append(opts, contextinfo.WithDeployRules(rules))
	}
	if set["env-name"] {
		opts = append(opts, contextinfo.WithDeployVar("env_name", *envName))
	}
	if set["build-type"] {
		opts = append(opts, contextinfo.WithDeployVar("build_type", *buildType))
	}
	info := contextinfo.Detect(opts...)

	ro := contextinfo.RenderOptions{Prefix: outPrefix, Explain: doExplain}
	switch outFormat {
	case "envvar":
		fmt.Print(info.EnvVars(ro))
	case "json":
		b, err := info.FlatJSON(ro)
		emit(b, err)
	case "text":
		fmt.Print(info.Text(ro))
	case "tfvars":
		fmt.Print(info.TFVarsHCL(ro))
	default:
		fmt.Fprintf(os.Stderr, "contextinfo: unknown format %q (want envvar, json, text, or tfvars)\n", outFormat)
		os.Exit(2)
	}
}

// resolveStr applies precedence default < config file < explicitly-set flag.
func resolveStr(def string, fromFile *string, flagSet bool, flagVal string) string {
	v := def
	if fromFile != nil {
		v = *fromFile
	}
	if flagSet {
		v = flagVal
	}
	return v
}

// resolveBool applies precedence default < config file < explicitly-set flag.
func resolveBool(def bool, fromFile *bool, flagSet bool, flagVal bool) bool {
	v := def
	if fromFile != nil {
		v = *fromFile
	}
	if flagSet {
		v = flagVal
	}
	return v
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
	// Fprint (not Fprintf) so the regex backslashes and any % are printed verbatim.
	fmt.Fprint(out, `
Config file (optional): .contextinfo.yaml (or .yml), searched in the cwd, parent
dirs up to the repo root, $HOME, then /etc/contextinfo.yaml — merged closest-wins.
Explicit flags override it. Keys mirror the flags (format, prefix, files_checksum,
explain), plus a deploy block that derives variables (env_name, build_type, …)
from the detected context. Example .contextinfo.yaml:

  format: envvar
  prefix: TF_VAR_
  deploy:
    rules:                          # first match wins
      - if:
          tag:
            regex: '^v[0-9]+\.[0-9]+\.[0-9]+$'   # strict semver tag
        set: { env_name: prod, build_type: production }
      - if:
          branch: main
        set: { env_name: prod, build_type: production }
      - if:
          branch: "release/*"       # bare strings are globs
        set: { env_name: dev, build_type: staging }
      - if:                         # (develop OR feature/*) AND not a PR
          all:
            - any:
                - { branch: develop }
                - { branch: "feature/*" }
            - not: { event: pull_request }
        set: { env_name: dev, build_type: development }
    default:
      set: { env_name: dev, build_type: development }
`)
}

// emit writes b to stdout, or exits non-zero on a rendering error.
func emit(b []byte, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "contextinfo: %v\n", err)
		os.Exit(1)
	}
	os.Stdout.Write(b)
}
