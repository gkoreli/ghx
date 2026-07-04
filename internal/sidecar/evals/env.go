package evals

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var requiredPlainTools = []string{"sh", "bash", "gh", "node", "npx", "npm"}

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

func buildPlainProfileEnv(base []string, binDir string) ([]string, error) {
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return nil, err
	}
	basePath := envValue(base, "PATH")
	for _, name := range requiredPlainTools {
		target, err := lookPathWithoutGhx(name, basePath)
		if err != nil {
			if name == "bash" {
				continue
			}
			return nil, fmt.Errorf("plain profile PATH: required tool %q not found: %w", name, err)
		}
		link := filepath.Join(binDir, name)
		if err := os.Symlink(target, link); err != nil && !os.IsExist(err) {
			return nil, fmt.Errorf("symlink %s -> %s: %w", link, target, err)
		}
	}
	if ghx, err := exec.LookPath("ghx"); err == nil {
		for _, part := range filepath.SplitList(binDir) {
			if sameDir(filepath.Dir(ghx), part) {
				return nil, fmt.Errorf("plain profile PATH unexpectedly includes ghx dir %s", part)
			}
		}
	}
	return setEnv(base, "PATH", binDir), nil
}

func lookPathWithoutGhx(name, pathValue string) (string, error) {
	for _, dir := range filepath.SplitList(pathValue) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			continue
		}
		if name != "ghx" {
			return candidate, nil
		}
	}
	return "", exec.ErrNotFound
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

func sameDir(a, b string) bool {
	ar, errA := filepath.EvalSymlinks(a)
	br, errB := filepath.EvalSymlinks(b)
	if errA == nil {
		a = ar
	}
	if errB == nil {
		b = br
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func agentIdentity(cfg RunConfig, initAgent *AgentIdentity) AgentIdentity {
	id := AgentIdentity{
		AgentCommand: cfg.AgentCmd,
		SubjectModel: os.Getenv("GHX_EVAL_SUBJECT_MODEL"),
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
