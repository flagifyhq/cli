package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// ProjectFileVersion is the on-disk schema version for .flagify/project.json.
	ProjectFileVersion = 1

	// ProjectDirname is the directory the project file lives inside, at the repo root.
	ProjectDirname = ".flagify"

	// ProjectFileBasename is the committable project file name — deliberately
	// distinct from the global ~/.flagify/config.json to avoid any walker ambiguity.
	ProjectFileBasename = "project.json"
)

// ProjectFileData is the committable JSON body describing a repo's Flagify scope.
// Never contains tokens. `PreferredProfile` is a hint — the CLI may use it when
// a matching local profile exists, but never requires it.
type ProjectFileData struct {
	Version          int    `json:"version"`
	WorkspaceID      string `json:"workspaceId,omitempty"`
	Workspace        string `json:"workspace,omitempty"`
	ProjectID        string `json:"projectId,omitempty"`
	Project          string `json:"project,omitempty"`
	Environment      string `json:"environment,omitempty"`
	PreferredProfile string `json:"preferredProfile,omitempty"`
}

// ProjectFile is a located project file — the parsed body plus the absolute path
// and its containing repo directory. Dir is the path used as the binding key.
type ProjectFile struct {
	Path string
	Dir  string
	Data ProjectFileData
}

// FindProjectFile walks upward from startDir looking for .flagify/project.json.
// Stops before $HOME so the global ~/.flagify/config.json is never reached. A
// missing file returns (nil, nil), not an error.
func FindProjectFile(startDir string) (*ProjectFile, error) {
	if startDir == "" {
		return nil, nil
	}
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return nil, err
	}

	home, _ := os.UserHomeDir()

	dir := abs
	for {
		// Guard: never look inside $HOME itself. This keeps the walker from
		// even considering ~/.flagify/, regardless of basename differences.
		if home != "" && dir == home {
			return nil, nil
		}

		candidate := filepath.Join(dir, ProjectDirname, ProjectFileBasename)
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return readProjectFile(candidate, dir)
		}
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// filesystem root reached
			return nil, nil
		}
		dir = parent
	}
}

func readProjectFile(path, dir string) (*ProjectFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pfd ProjectFileData
	if err := json.Unmarshal(data, &pfd); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return &ProjectFile{Path: path, Dir: dir, Data: pfd}, nil
}

// WriteProjectFile creates or overwrites .flagify/project.json under dir.
// The write is atomic (temp + rename). Tokens are never accepted — the caller
// must construct ProjectFileData explicitly, so there is no accidental leak path.
func WriteProjectFile(dir string, pfd ProjectFileData) (*ProjectFile, error) {
	if dir == "" {
		return nil, fmt.Errorf("empty dir")
	}
	if pfd.Version == 0 {
		pfd.Version = ProjectFileVersion
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	flagifyDir := filepath.Join(abs, ProjectDirname)
	if err := os.MkdirAll(flagifyDir, 0o755); err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(pfd, "", "  ")
	if err != nil {
		return nil, err
	}

	path := filepath.Join(flagifyDir, ProjectFileBasename)

	tmp, err := os.CreateTemp(flagifyDir, ".project.json.*.tmp")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	writeOK := false
	defer func() {
		if !writeOK {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return nil, err
	}
	writeOK = true

	return &ProjectFile{Path: path, Dir: abs, Data: pfd}, nil
}
