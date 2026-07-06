package evals

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// BaselineReuseEnv names the prior committed run directory whose plain and
	// ghx episodes may be copied into the current run when ADR-0025.1 identity
	// checks pass.
	BaselineReuseEnv = "GHX_EVAL_BASELINE_REUSE_RUN_DIR"
	// BaselineFallbackEnv permits an explicit fresh-baseline fallback when a
	// requested baseline-reuse source fails an eligibility check.
	BaselineFallbackEnv = "GHX_EVAL_BASELINE_FALLBACK"
	// BaselineFallbackFresh is the only fallback mode that allows fresh
	// baselines after a requested reuse source is refused.
	BaselineFallbackFresh = "fresh"
	// BaselineReuseLabel is the verdict/manifest label for a run whose
	// baseline episodes were copied from a prior run.
	BaselineReuseLabel = "BASELINE-REUSED"
	// BaselineReuseMaxAgeDays is the ADR-0025 D2 age bound for candidate runs.
	BaselineReuseMaxAgeDays = 7
)

type episodeFile struct {
	Path string
	Data []byte
	Ep   *Episode
}

// TryReuseBaselines validates a prior run against ADR-0025.1 and, on success,
// copies its plain and ghx episode JSONs byte-for-byte into runDir. A mismatch
// returns ok=false with a first reason and no baselineReuse manifest block.
func TryReuseBaselines(runDir, priorRunDir string, cfg RunConfig, tasks []Task, taskDir string, plannedTrials int, identity AgentIdentity, now time.Time) (*BaselineReuse, bool, string, error) {
	if strings.TrimSpace(priorRunDir) == "" {
		return nil, false, "baseline reuse disabled: no prior run directory configured", nil
	}
	if plannedTrials <= 0 {
		return nil, false, "baseline reuse disabled: planned trial count must be positive", nil
	}
	currentHashes, reason, err := BaselineReuseHashes(cfg, tasks, taskDir, identity)
	if err != nil {
		return nil, false, reason, err
	}
	if reason != "" {
		return nil, false, reason, nil
	}

	prior, err := LoadRunManifest(priorRunDir)
	if err != nil {
		return nil, false, "baseline reuse disabled: prior manifest unreadable", err
	}
	if prior == nil {
		return nil, false, "baseline reuse disabled: prior manifest missing", nil
	}
	if prior.CreatedAt.IsZero() {
		return nil, false, "baseline reuse disabled: prior manifest createdAt missing", nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if now.UTC().Sub(prior.CreatedAt.UTC()) > BaselineReuseMaxAgeDays*24*time.Hour {
		return nil, false, "baseline reuse disabled: prior run older than 7 days", nil
	}
	if len(prior.IdentityHashes) == 0 {
		return nil, false, "baseline reuse disabled: prior run lacks manifest identity hash inventory; rerun the source after 2026-07-06 baseline-reuse fix so manifest.identityHashes is written at run start", nil
	}
	if prior.Identity.AdapterVersion != identity.AdapterVersion {
		return nil, false, "baseline reuse disabled: prior manifest adapterVersion mismatch", nil
	}
	if mismatch := compareHashInventory(currentHashes, prior.IdentityHashes); mismatch != "" {
		return nil, false, "baseline reuse disabled: " + mismatch, nil
	}

	files, err := loadRunEpisodeFiles(priorRunDir)
	if err != nil {
		return nil, false, "baseline reuse disabled: prior episodes unreadable", err
	}
	taskByID := map[string]Task{}
	for _, task := range tasks {
		taskByID[task.ID] = task
	}
	matrix := map[string][]episodeFile{}
	seenTarget := map[string]bool{}
	wrapperSHA := hashValue(currentHashes, "wrapperScript")
	subjectModel := identity.SubjectModel
	if subjectModel == "" {
		subjectModel = resolveSubjectModel(cfg.AgentCmd)
	}
	for _, f := range files {
		ep := f.Ep
		if ep.Profile != ProfilePlain && ep.Profile != ProfileGhx {
			continue
		}
		task, ok := taskByID[ep.TaskID]
		if !ok {
			return nil, false, fmt.Sprintf("baseline reuse disabled: prior episode %s taskId %q not in current corpus", ep.ID, ep.TaskID), nil
		}
		if ep.Repo != task.Repo {
			return nil, false, fmt.Sprintf("baseline reuse disabled: prior episode %s repo mismatch", ep.ID), nil
		}
		if ep.Identity.WrapperSHA256 != wrapperSHA {
			return nil, false, fmt.Sprintf("baseline reuse disabled: prior episode %s wrapper hash mismatch", ep.ID), nil
		}
		if ep.Identity.AdapterVersion != identity.AdapterVersion {
			return nil, false, fmt.Sprintf("baseline reuse disabled: prior episode %s adapterVersion mismatch", ep.ID), nil
		}
		if ep.Identity.SubjectModel != subjectModel {
			return nil, false, fmt.Sprintf("baseline reuse disabled: prior episode %s subjectModel mismatch", ep.ID), nil
		}
		if ep.Invalid || len(ep.ExclusionReasons) > 0 {
			continue
		}
		target := filepath.Join(runDir, filepath.Base(f.Path))
		if seenTarget[target] {
			return nil, false, fmt.Sprintf("baseline reuse disabled: target filename collision for %s", filepath.Base(f.Path)), nil
		}
		if existing, err := os.ReadFile(target); err == nil {
			if !bytes.Equal(existing, f.Data) {
				return nil, false, fmt.Sprintf("baseline reuse disabled: target filename already exists with different bytes: %s", target), nil
			}
		} else if err != nil && !os.IsNotExist(err) {
			return nil, false, "baseline reuse disabled: target filename stat failed", err
		}
		seenTarget[target] = true
		key := ep.TaskID + "\x00" + string(ep.Profile)
		matrix[key] = append(matrix[key], f)
	}

	for _, task := range tasks {
		for _, profile := range []Profile{ProfilePlain, ProfileGhx} {
			key := task.ID + "\x00" + string(profile)
			if got := len(matrix[key]); got < plannedTrials {
				return nil, false, fmt.Sprintf("baseline reuse disabled: source cell has too few valid trials task=%s profile=%s got=%d want>=%d; add valid non-excluded baseline trials to the source run or lower the planned run total", task.ID, profile, got, plannedTrials), nil
			}
			sort.Slice(matrix[key], func(i, j int) bool {
				left, right := matrix[key][i].Ep.StartedAt, matrix[key][j].Ep.StartedAt
				if left.Equal(right) {
					return filepath.Base(matrix[key][i].Path) < filepath.Base(matrix[key][j].Path)
				}
				if left.IsZero() {
					return false
				}
				if right.IsZero() {
					return true
				}
				return left.Before(right)
			})
			matrix[key] = matrix[key][:plannedTrials]
		}
	}

	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return nil, false, "", fmt.Errorf("mkdir %s: %w", runDir, err)
	}
	reuse := &BaselineReuse{
		ReusedFromRunID:  filepath.Base(filepath.Clean(priorRunDir)),
		ReusedFromRunDir: priorRunDir,
		ReusedAt:         now.UTC(),
		MaxAgeDays:       BaselineReuseMaxAgeDays,
		VerdictLabel:     BaselineReuseLabel,
		Hashes:           currentHashes,
	}
	var toCopy []episodeFile
	for _, fs := range matrix {
		toCopy = append(toCopy, fs...)
	}
	sort.Slice(toCopy, func(i, j int) bool {
		return filepath.Base(toCopy[i].Path) < filepath.Base(toCopy[j].Path)
	})
	for _, f := range toCopy {
		target := filepath.Join(runDir, filepath.Base(f.Path))
		if existing, err := os.ReadFile(target); err == nil {
			if !bytes.Equal(existing, f.Data) {
				return nil, false, fmt.Sprintf("baseline reuse disabled: target filename already exists with different bytes: %s", target), nil
			}
		} else if os.IsNotExist(err) {
			if err := os.WriteFile(target, f.Data, 0o644); err != nil {
				return nil, false, "", fmt.Errorf("copy reused baseline %s: %w", f.Path, err)
			}
		} else {
			return nil, false, "", fmt.Errorf("read copied baseline %s: %w", target, err)
		}
		copied, err := os.ReadFile(target)
		if err != nil {
			return nil, false, "", fmt.Errorf("read copied baseline %s: %w", target, err)
		}
		reuse.Episodes = append(reuse.Episodes, BaselineReusedEpisodeRecord{
			Profile:    f.Ep.Profile,
			TaskID:     f.Ep.TaskID,
			SourceFile: f.Path,
			TargetFile: target,
			SHA256:     sha256Hex(copied),
		})
	}
	return reuse, true, "", nil
}

// PlannedBaselineTrials returns the manifest's planned valid trial count per
// baseline task/profile cell. Gate runs append one round per invocation, so
// reuse eligibility must compare against the accumulated planned total.
func PlannedBaselineTrials(m *RunManifest) int {
	if m == nil {
		return 0
	}
	total := 0
	for _, round := range m.Rounds {
		if round.Trials > 0 {
			total += round.Trials
		}
	}
	return total
}

// BaselineReuseRefusalMessage is the loud, actionable error emitted when a
// caller requested reuse and the source failed an eligibility check.
func BaselineReuseRefusalMessage(priorRunDir, reason string) string {
	return fmt.Sprintf("baseline reuse requested from %s but refused: %s\nremediation: use a source run whose manifest.identityHashes match and whose plain/ghx cells contain at least the planned valid trial count, or set %s=%s to explicitly run fresh baselines", priorRunDir, reason, BaselineFallbackEnv, BaselineFallbackFresh)
}

// BaselineFreshFallbackAllowed reports whether the operator explicitly allowed
// fresh baselines after a requested baseline-reuse source was refused.
func BaselineFreshFallbackAllowed() bool {
	return os.Getenv(BaselineFallbackEnv) == BaselineFallbackFresh
}

// BaselineReuseHashes builds the five ADR-0025.1 D1 hash records for the
// current run. It returns a non-empty reason when the current identity is not
// eligible for reuse.
func BaselineReuseHashes(cfg RunConfig, tasks []Task, taskDir string, identity AgentIdentity) ([]BaselineReuseHashRecord, string, error) {
	wrapper, ok := wrapperHash(cfg.AgentCmd)
	if !ok {
		return nil, "baseline reuse disabled: agent command is not a readable wrapper path", nil
	}
	adapterVersion := strings.TrimSpace(identity.AdapterVersion)
	if adapterVersion == "" {
		return nil, "baseline reuse disabled: adapter version is blank", nil
	}
	subjectModel := strings.TrimSpace(identity.SubjectModel)
	if subjectModel == "" {
		subjectModel = resolveSubjectModel(cfg.AgentCmd)
	}
	if subjectModel == "" || subjectModel == "unknown" {
		return nil, "baseline reuse disabled: subject model is unknown", nil
	}
	taskHash, err := taskCorpusHash(taskDir)
	if err != nil {
		return nil, "baseline reuse disabled: task corpus hash failed", err
	}
	preambleHash := directProfilePreambleHash(tasks)
	return []BaselineReuseHashRecord{
		{Name: "wrapperScript", Algorithm: "sha256", Value: wrapper, Source: cfg.AgentCmd, SourceKind: "file"},
		{Name: "adapterVersion", Algorithm: "sha256", Value: sha256Hex([]byte(adapterVersion)), Source: "AgentIdentity.AdapterVersion", SourceKind: "value"},
		{Name: "subjectModel", Algorithm: "sha256", Value: sha256Hex([]byte(subjectModel)), Source: "resolveSubjectModel(RunConfig.AgentCmd)", SourceKind: "value"},
		{Name: "taskCorpus", Algorithm: "sha256", Value: taskHash, Source: filepath.Join(taskDir, "*.json"), SourceKind: "file"},
		{Name: "directProfilePreamble", Algorithm: "sha256", Value: preambleHash, Source: "directPreamble(ProfilePlain, repo) and directPreamble(ProfileGhx, repo)", SourceKind: "generated"},
	}, "", nil
}

func taskCorpusHash(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		h.Write([]byte("file\x00"))
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write([]byte(strconv.Itoa(len(data))))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte("\n"))
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func directProfilePreambleHash(tasks []Task) string {
	h := sha256.New()
	for _, task := range tasks {
		for _, profile := range []Profile{ProfilePlain, ProfileGhx} {
			preamble := directPreamble(profile, task.Repo)
			h.Write([]byte("profile\x00"))
			h.Write([]byte(profile))
			h.Write([]byte("\x00task\x00"))
			h.Write([]byte(task.ID))
			h.Write([]byte("\x00repo\x00"))
			h.Write([]byte(task.Repo))
			h.Write([]byte("\x00len\x00"))
			h.Write([]byte(strconv.Itoa(len([]byte(preamble)))))
			h.Write([]byte{0})
			h.Write([]byte(preamble))
			h.Write([]byte("\n"))
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func loadRunEpisodeFiles(runDir string) ([]episodeFile, error) {
	entries, err := os.ReadDir(runDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" && !strings.HasPrefix(e.Name(), "verdict") && e.Name() != "manifest.json" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var files []episodeFile
	for _, name := range names {
		path := filepath.Join(runDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		ep, err := LoadEpisode(path)
		if err != nil {
			return nil, err
		}
		files = append(files, episodeFile{Path: path, Data: data, Ep: ep})
	}
	return files, nil
}

func compareHashInventory(current, prior []BaselineReuseHashRecord) string {
	priorByName := map[string]BaselineReuseHashRecord{}
	for _, rec := range prior {
		priorByName[rec.Name] = rec
	}
	for _, rec := range current {
		prev, ok := priorByName[rec.Name]
		if !ok {
			return "prior hash inventory missing " + rec.Name
		}
		if prev.Algorithm != rec.Algorithm || prev.Value != rec.Value || prev.SourceKind != rec.SourceKind {
			return "hash mismatch for " + rec.Name
		}
	}
	return ""
}

func hashValue(records []BaselineReuseHashRecord, name string) string {
	for _, rec := range records {
		if rec.Name == name {
			return rec.Value
		}
	}
	return ""
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}
