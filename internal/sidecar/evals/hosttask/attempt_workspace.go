package hosttask

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// CopyWorkspacePerAttempt is the production Grader.AttemptWorkspace seam
// (ADR-0032.1 S3): it materializes a pristine copy of the graded workspace —
// the host's fixed tree — in a fresh temporary directory for one grade
// attempt, and returns a cleanup that removes the copy. Because every
// attempt (first pass and trap-7 flake re-runs alike) starts from the same
// bytes, a test that mutates the tree (caches, marker files) cannot flip a
// later attempt's outcome and convert a deterministic failure into a false
// Flaky disqualification, and the source workspace stays exactly what the
// host produced (auditable evidence).
func CopyWorkspacePerAttempt(_ context.Context, workspaceDir string, attempt int) (string, func(), error) {
	dir, err := os.MkdirTemp("", fmt.Sprintf("ghx-grade-attempt%02d-*", attempt))
	if err != nil {
		return "", nil, fmt.Errorf("create attempt workspace: %w", err)
	}
	if err := copyTree(workspaceDir, dir); err != nil {
		os.RemoveAll(dir)
		return "", nil, fmt.Errorf("copy workspace for attempt %d: %w", attempt, err)
	}
	return dir, func() { os.RemoveAll(dir) }, nil
}

// copyTree copies src into dst (which must exist and be empty): directories
// with their permission bits, regular files with contents and permission
// bits, and symlinks as symlinks (git checkouts may contain them; their
// targets are copied verbatim, relative or not). Any other file type is an
// error — a grade attempt must never run against a partially copied tree.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if rel == "." {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return os.Mkdir(target, info.Mode().Perm())
		case info.Mode()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case info.Mode().IsRegular():
			return copyFile(path, target, info.Mode().Perm())
		default:
			return fmt.Errorf("unsupported file type %s in workspace: %s", info.Mode().Type(), path)
		}
	})
}

// copyFile copies one regular file with the given permission bits.
func copyFile(src, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
