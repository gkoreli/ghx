package evals

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var wrapperModelPatterns = []*regexp.Regexp{
	regexp.MustCompile(`GHX_EVAL_SUBJECT_MODEL="\$\{GHX_EVAL_SUBJECT_MODEL:-([^}]+)\}"`),
	regexp.MustCompile(`GHX_EVAL_SUBJECT_MODEL=\$\{GHX_EVAL_SUBJECT_MODEL:-([^}]+)\}`),
	regexp.MustCompile(`ANTHROPIC_MODEL="?\$\{ANTHROPIC_MODEL:-([^}"]+)\}"?`),
	regexp.MustCompile(`ANTHROPIC_MODEL="?([^"\s]+)"?`),
}

type episodeRuntime struct {
	Cwd     string
	Env     []string
	Cleanup func()
}

func prepareEpisodeRuntime(cfg RunConfig, profile Profile) (episodeRuntime, error) {
	root, err := os.MkdirTemp("", "ghx-eval-episode-*")
	if err != nil {
		return episodeRuntime{}, err
	}
	rt := episodeRuntime{
		Cwd: root,
		Env: os.Environ(),
		Cleanup: func() {
			_ = os.RemoveAll(root)
		},
	}
	if profile != ProfilePlain {
		return rt, nil
	}
	env, err := buildPlainProfileEnv(os.Environ(), filepath.Join(root, "bin"))
	if err != nil {
		rt.Cleanup()
		return episodeRuntime{}, err
	}
	rt.Env = env
	return rt, nil
}

func buildPlainProfileEnv(base []string, _ string) ([]string, error) {
	basePath := envValue(base, "PATH")
	var kept []string
	for _, dir := range filepath.SplitList(basePath) {
		if dir == "" {
			continue
		}
		hasGhx, err := dirProvidesGhx(dir)
		if err != nil {
			return nil, err
		}
		if hasGhx {
			continue
		}
		kept = append(kept, dir)
	}
	return setEnv(base, "PATH", strings.Join(kept, string(os.PathListSeparator))), nil
}

func dirProvidesGhx(dir string) (bool, error) {
	for _, name := range []string{"ghx", "ghx.exe"} {
		candidate := filepath.Join(dir, name)
		info, err := os.Stat(candidate)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, fmt.Errorf("plain profile PATH: stat %s: %w", candidate, err)
		}
		if info.IsDir() || info.Mode()&0o111 == 0 {
			continue
		}
		return true, nil
	}
	return false, nil
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return strings.TrimPrefix(kv, prefix)
		}
	}
	return ""
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	found := false
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			out = append(out, prefix+value)
			found = true
		} else {
			out = append(out, kv)
		}
	}
	if !found {
		out = append(out, prefix+value)
	}
	return out
}

func agentIdentity(cfg RunConfig, initAgent *AgentIdentity) AgentIdentity {
	id := AgentIdentity{
		AgentCommand: cfg.AgentCmd,
		SubjectModel: resolveSubjectModel(cfg.AgentCmd),
	}
	if initAgent != nil {
		id.AdapterName = initAgent.AdapterName
		id.AdapterVersion = initAgent.AdapterVersion
		id.AdapterSubjectModel = initAgent.AdapterSubjectModel
	}
	if hash, ok := wrapperHash(cfg.AgentCmd); ok {
		id.WrapperSHA256 = hash
	}
	return id
}

func resolveSubjectModel(agentCmd string) string {
	if v := strings.TrimSpace(os.Getenv("GHX_EVAL_SUBJECT_MODEL")); v != "" {
		return v
	}
	if v, ok := subjectModelFromWrapper(agentCmd); ok {
		return v
	}
	return "unknown"
}

func subjectModelFromWrapper(cmd string) (string, bool) {
	if cmd == "" || (!filepath.IsAbs(cmd) && !strings.Contains(cmd, string(os.PathSeparator))) {
		return "", false
	}
	data, err := os.ReadFile(cmd)
	if err != nil {
		return "", false
	}
	text := string(data)
	for _, re := range wrapperModelPatterns {
		m := re.FindStringSubmatch(text)
		if len(m) < 2 {
			continue
		}
		model := strings.Trim(m[1], `"' `)
		if model == "$ANTHROPIC_MODEL" {
			if v, ok := anthropicModelDefault(text); ok {
				return v, true
			}
			continue
		}
		if model != "" {
			return model, true
		}
	}
	return "", false
}

func anthropicModelDefault(text string) (string, bool) {
	for _, re := range wrapperModelPatterns[2:] {
		m := re.FindStringSubmatch(text)
		if len(m) >= 2 {
			model := strings.Trim(m[1], `"' `)
			if model != "" && !strings.HasPrefix(model, "$") {
				return model, true
			}
		}
	}
	return "", false
}

func wrapperHash(cmd string) (string, bool) {
	if cmd == "" || (!filepath.IsAbs(cmd) && !strings.Contains(cmd, string(os.PathSeparator))) {
		return "", false
	}
	data, err := os.ReadFile(cmd)
	if err != nil {
		return "", false
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:]), true
}
