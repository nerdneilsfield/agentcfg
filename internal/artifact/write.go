package artifact

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// WriteFiles prepares every merge before touching a destination. Each file is
// replaced by a same-directory rename; a filesystem failure is not a multi-file
// transaction and is reported with the failing path.
func WriteFiles(arts []Artifact, targetCount int, output string, inPlace bool, log io.Writer) error {
	arts = companions(arts)
	if inPlace && output != "" {
		return fmt.Errorf("--in-place and --output are mutually exclusive")
	}
	if !inPlace && output == "" {
		return fmt.Errorf("output path is required")
	}
	directory := targetCount > 1 || len(arts) > 1 || strings.HasSuffix(output, string(os.PathSeparator))
	if info, err := os.Stat(output); err == nil && info.IsDir() {
		directory = true
	}
	type pending struct {
		path    string
		content []byte
		mode    os.FileMode
	}
	plans := []pending{}
	seen := map[string]int{}
	raw := map[string][]byte{}
	for _, a := range arts {
		var path string
		if inPlace {
			path = a.SuggestedPath
			if a.Target == "deepseek-harness" && a.Name == "providers.patch.yaml" {
				path = "$DSH_HOME/settings.yaml"
			}
			var err error
			path, err = expandPath(path)
			if err != nil {
				return err
			}
		} else if directory {
			name := a.Name
			if a.Target == "fast-agent" && name != "fastagent.config.yaml" {
				name = filepath.Join(".fast-agent", name)
			}
			if a.Target == "goose" && name != "config.yaml" {
				name = filepath.Join("custom_providers", name)
			}
			if a.Target == "deepseek-harness" && name == "providers.patch.yaml" {
				name = "settings.yaml"
			}
			if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(filepath.Clean(name), ".."+string(os.PathSeparator)) {
				return fmt.Errorf("invalid artifact path %q", name)
			}
			if targetCount > 1 {
				name = filepath.Join(a.Target, name)
			}
			path = filepath.Join(output, name)
		} else {
			path = output
		}
		path, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if _, ok := seen[path]; ok {
			if bytes.Equal(raw[path], a.Content) {
				continue
			}
			return fmt.Errorf("multiple artifacts target %s; select one target or use --output directory", path)
		}
		mode := os.FileMode(0o600)
		content := a.Content
		info, err := os.Lstat(path)
		if err == nil {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("%s is not a regular file", path)
			}
			mode = info.Mode().Perm()
			old, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			content, err = merge(a, old)
			if err != nil {
				return fmt.Errorf("merging %s: %w", path, err)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		seen[path] = len(plans)
		raw[path] = a.Content
		plans = append(plans, pending{path, content, mode})
	}
	// Stage all files before the first rename, including permission checks and
	// directory creation. Temporary files contain credentials and start private.
	temps := make([]string, len(plans))
	defer func() {
		for _, p := range temps {
			if p != "" {
				_ = os.Remove(p)
			}
		}
	}()
	for i, p := range plans {
		if err := os.MkdirAll(filepath.Dir(p.path), 0o700); err != nil {
			return err
		}
		f, err := os.CreateTemp(filepath.Dir(p.path), ".agentcfg-*")
		if err != nil {
			return err
		}
		temps[i] = f.Name()
		_, writeErr := f.Write(p.content)
		if writeErr == nil {
			writeErr = f.Chmod(p.mode)
		}
		if writeErr == nil {
			writeErr = f.Sync()
		}
		closeErr := f.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	for i, p := range plans {
		if err := os.Rename(temps[i], p.path); err != nil {
			return fmt.Errorf("writing %s: %w", p.path, err)
		}
		temps[i] = ""
		_, _ = fmt.Fprintf(log, "wrote %s\n", p.path)
	}
	return nil
}

func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	}
	var missing string
	path = os.Expand(path, func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			missing = key
		}
		return v
	})
	if missing != "" {
		return "", fmt.Errorf("native path requires environment variable %s", missing)
	}
	if path == "" {
		return "", fmt.Errorf("empty native path")
	}
	return path, nil
}
