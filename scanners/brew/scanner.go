package brew

import (
	"bufio"
	"bytes"
	"os/exec"
	"strings"
)

// OrphanedDep is a Homebrew formula that was installed as a dependency
// but whose parent formula/cask has since been removed.
type OrphanedDep struct {
	Name    string
	Version string
	UsedBy  []string // currently empty → no remaining dependents
}

// FindOrphanedDeps returns formulae that are leaves in the dependency graph
// but were not explicitly installed by the user.
func FindOrphanedDeps() ([]OrphanedDep, error) {
	// Get all installed formulae
	out, err := exec.Command("brew", "list", "--formula", "-1").Output()
	if err != nil {
		return nil, err
	}

	var formulae []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if f := strings.TrimSpace(sc.Text()); f != "" {
			formulae = append(formulae, f)
		}
	}

	// Get all explicitly requested formulae (installed by user, not as deps)
	requested, err := explicitlyInstalled()
	if err != nil {
		return nil, err
	}

	// A formula is orphaned if it is not explicitly requested AND nothing
	// installed depends on it.
	var orphans []OrphanedDep
	for _, f := range formulae {
		if requested[f] {
			continue
		}
		users := dependents(f)
		if len(users) == 0 {
			orphans = append(orphans, OrphanedDep{Name: f, UsedBy: users})
		}
	}
	return orphans, nil
}

func explicitlyInstalled() (map[string]bool, error) {
	out, err := exec.Command("brew", "leaves").Output()
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool)
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if f := strings.TrimSpace(sc.Text()); f != "" {
			m[f] = true
		}
	}
	return m, nil
}

func dependents(formula string) []string {
	out, err := exec.Command("brew", "uses", "--installed", formula).Output()
	if err != nil {
		return nil
	}
	var deps []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if d := strings.TrimSpace(sc.Text()); d != "" {
			deps = append(deps, d)
		}
	}
	return deps
}
