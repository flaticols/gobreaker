package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/flaticols/gobreaker/internal/git"
	"github.com/flaticols/gobreaker/pkg/breaking"
	"github.com/jessevdk/go-flags"
)

type programOptions struct {
	//nolint:golines
	RepoPath string `short:"r" long:"repo" description:"Path to git repository (default: current directory)"`
	//nolint:golines
	Output          string `short:"f" long:"format" description:"Output format (text, json, markdown)" default:"text" choice:"text"`
	IncludeInternal bool   `short:"i" long:"include-internal" description:"Include internal packages in API analysis"`
	Quite           bool   `short:"q" long:"quiet" description:"Suppress output"`
	Version         bool   `short:"v" long:"version" description:"Print version information and exit"`

	// Positional arguments
	Args struct {
		OldRef string `positional-arg-name:"old-ref" description:"Old reference (branch, tag, or commit) to compare from"`
		NewRef string `positional-arg-name:"new-ref" description:"New reference (branch, tag, or commit) to compare to (default: HEAD)"`
	} `positional-args:"yes"`
}

func main() {
	programCfg := programOptions{}
	p := flags.NewParser(&programCfg, flags.Default)

	args, err := p.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}

	if programCfg.Version {
		printVersion()
		os.Exit(0)
	}

	// Handle positional arguments
	oldRef := programCfg.Args.OldRef
	newRef := programCfg.Args.NewRef

	if oldRef == "" {
		_, _ = fmt.Fprintf(os.Stderr, "Error: old-ref is required\n")
		_, _ = fmt.Fprintf(os.Stderr, "Usage: gobreaker [OPTIONS] <old-ref> [new-ref]\n")
		_, _ = fmt.Fprintf(os.Stderr, "       gobreaker [OPTIONS] <old-path> <new-path>\n")
		_, _ = fmt.Fprintf(os.Stderr, "Run 'gobreaker --help' for more information\n")
		os.Exit(1)
	}

	if newRef == "" {
		newRef = "HEAD"
	}

	if programCfg.RepoPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		programCfg.RepoPath = wd
	}

	if len(args) > 0 {
		_, _ = fmt.Fprintf(os.Stderr, "Error: unexpected arguments: %v\n", args)
		os.Exit(1)
	}

	var diff *breaking.Diff

	// Auto-detect: filesystem paths or git refs
	oldIsPath := git.IsFilesystemPath(oldRef)
	newIsPath := git.IsFilesystemPath(newRef)

	if oldIsPath && newIsPath {
		// Both are filesystem paths - compare directories directly
		diff, err = git.CompareFilesystems(oldRef, newRef, programCfg.IncludeInternal)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	} else if !oldIsPath && !newIsPath {
		// Both are git refs - use git mode
		diff, err = git.OpenRepo(programCfg.RepoPath, oldRef, newRef, programCfg.IncludeInternal)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Mixed mode not supported
		_, _ = fmt.Fprintf(os.Stderr, "Error: cannot mix filesystem paths and git refs\n")
		_, _ = fmt.Fprintf(os.Stderr, "Use either two filesystem paths or two git refs\n")
		os.Exit(1)
	}

	if !programCfg.Quite {
		err = diff.Reports()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	if !diff.IsCompatible() {
		os.Exit(1)
	}

	os.Exit(0)
}

func printVersion() {
	if info, ok := debug.ReadBuildInfo(); ok {
		fmt.Printf("gobreaker version %s\n", info.Main.Version)
		fmt.Printf("go version %s\n", info.GoVersion)
		for _, dep := range info.Deps {
			if dep.Path == "golang.org/x/exp" {
				fmt.Printf("apidiff version %s\n", dep.Version)
				break
			}
		}
	} else {
		fmt.Println("gobreaker version (devel)")
	}
}
