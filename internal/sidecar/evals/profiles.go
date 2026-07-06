package evals

import "fmt"

// Profile identifies one of the three workflow architectures under comparison
// (ADR-0016): plain baseline, direct ghx usage, and delegated sidecar.
type Profile string

const (
	// ProfilePlain is a general agent with its own common tools (shell, gh).
	ProfilePlain Profile = "plain"
	// ProfileGhx is a general agent instructed to explore with ghx directly.
	ProfileGhx Profile = "ghx"
	// ProfileSidecar delegates reconnaissance through sidecar.Ask — the
	// exact production code path behind "ghx sidecar ask".
	ProfileSidecar Profile = "ghx-sidecar"
)

// AllProfiles returns the three profiles in canonical comparison order.
func AllProfiles() []Profile {
	return []Profile{ProfilePlain, ProfileGhx, ProfileSidecar}
}

// directPreamble builds the first-turn instruction block for the two direct
// profiles. The sidecar profile does not use this — its persona comes from
// sidecar.BuildPrompt so the eval measures the production prompt.
func directPreamble(p Profile, repo string) string {
	return directPreambleBuilder(p, repo)
}

var directPreambleBuilder = defaultDirectPreamble

func defaultDirectPreamble(p Profile, repo string) string {
	if repo == "" {
		return defaultDiscoveryDirectPreamble(p)
	}
	common := fmt.Sprintf(`You are investigating the GitHub repository %s.
Work remotely — do NOT clone the repository and do NOT write any files.
Answer the question below with the specific implementation files (exact paths),
the key functions or symbols involved, and evidence for each claim.
State what you did not verify.`, repo)

	switch p {
	case ProfilePlain:
		return common + `
You may use your shell and the gh CLI to inspect the repository.`
	case ProfileGhx:
		return common + `
Use the ghx CLI for all repository access. Core commands:
  ghx explore <owner/repo>              — branch + tree + README in 1 call
  ghx tree <owner/repo> [path] --depth N
  ghx read <owner/repo> <file> [file...]         — up to 10 files per call
  ghx read <owner/repo> "src/**/*.ts" --map      — structural map (signatures)
  ghx read <owner/repo> --grep "pattern" <path>  — matching lines only
  ghx search "<query> repo:<owner/repo>"         — code search
Map before reading. Narrow globs before reading many files.`
	default:
		return common
	}
}

func defaultDiscoveryDirectPreamble(p Profile) string {
	common := `You are discovering open-source GitHub repositories that answer the question.
Work remotely — do NOT clone repositories and do NOT write any files.
Sweep broadly, then verify repository claims by reading concrete files before calling them verified.
Use owner/repo or owner/repo:path citations. Repositories seen only in search results or metadata are inferred or unverified, not verified.
State what you did not verify.`

	switch p {
	case ProfilePlain:
		return common + `
You may use your shell, gh search, gh repo view, gh api contents, and raw GitHub file reads.`
	case ProfileGhx:
		return common + `
Use the ghx CLI for GitHub exploration. Core commands:
  ghx search "<query>"                 — cross-GitHub code or repo search
  ghx repos "<query>"                  — repository discovery
  ghx explore <owner/repo>             — branch + tree + README in 1 call
  ghx read <owner/repo> <file>         — verify a concrete path
Search broadly first, then read into the best candidates before claiming them as verified.`
	default:
		return common
	}
}

// directPrompt returns the full prompt for one turn of a direct profile.
// Turn 0 carries the preamble; follow-ups send the bare question because the
// live ACP session already holds the context.
func directPrompt(p Profile, repo, question string, turn int) string {
	if turn == 0 {
		return directPreamble(p, repo) + "\n\nQuestion: " + question
	}
	return question
}
