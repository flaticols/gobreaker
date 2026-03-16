package breaking

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/exp/apidiff"
)

type DiffReport = map[string]apidiff.Report

// Diff represents the API differences between two versions of Go packages.
// It tracks both regular package changes and import changes, determining
// overall compatibility based on the presence of breaking changes.
type Diff struct {
	apiChangeReports DiffReport
	importsChange    DiffReport
	breakingImports  bool
	incompatible     bool
}

func New(apiChanges, importChanges DiffReport) *Diff {
	return &Diff{
		apiChangeReports: apiChanges,
		importsChange:    importChanges,
	}
}

func (d *Diff) SetBreakingImports(b bool) {
	d.breakingImports = b
}

func (d *Diff) SetIncompatible(b bool) {
	d.incompatible = b
}

// IsCompatible returns true if there are no breaking changes in either
// the package APIs or imports.
func (d *Diff) IsCompatible() bool {
	return !d.breakingImports && !d.incompatible
}

// HasChanges returns true if any API changes (compatible or incompatible) were detected.
func (d *Diff) HasChanges() bool {
	for _, r := range d.apiChangeReports {
		if len(r.Changes) > 0 {
			return true
		}
	}
	for _, r := range d.importsChange {
		if len(r.Changes) > 0 {
			return true
		}
	}
	return false
}

// WriteText writes all API diff reports to the given writer with proper formatting.
func (d *Diff) WriteText(out io.Writer) error {
	w := &TextDiffReport{prefix: "  ", w: out}
	for pkg, report := range d.apiChangeReports {
		if err := writeReport(out, w, pkg, report); err != nil {
			return err
		}
	}
	for pkg, report := range d.importsChange {
		if err := writeReport(out, w, pkg, report); err != nil {
			return err
		}
	}
	return nil
}

// Reports writes all API diff reports to stdout.
func (d *Diff) Reports() error {
	return d.WriteText(os.Stdout)
}

func writeReport(out, w io.Writer, name string, report apidiff.Report) error {
	var (
		hasIncompatible bool
		hasCompatible   bool
	)

	for _, c := range report.Changes {
		if !c.Compatible {
			hasIncompatible = true
		} else {
			hasCompatible = true
		}
	}

	if hasIncompatible {
		if _, err := fmt.Fprintf(out, "\n%s\n", name); err != nil {
			return err
		}
		if err := report.TextIncompatible(w, true); err != nil {
			return err
		}
	}

	if hasCompatible {
		if _, err := fmt.Fprintf(out, "\n%s\n", name); err != nil {
			return err
		}
		if err := report.TextCompatible(w); err != nil {
			return err
		}
	}
	return nil
}
