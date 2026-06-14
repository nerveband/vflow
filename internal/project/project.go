package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Project struct {
	Version   string    `json:"version"`
	ID        string    `json:"id"`
	Root      string    `json:"root"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type InitResult struct {
	Status       string   `json:"status"`
	Project      Project  `json:"project"`
	PlannedPaths []string `json:"planned_paths"`
	ChangedPaths []string `json:"changed_paths"`
}

func Init(path, id string, commit bool) (InitResult, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return InitResult{}, err
	}
	now := time.Now().UTC()
	proj := Project{Version: "vflow-project/v1", ID: id, Root: abs, CreatedAt: now, UpdatedAt: now}
	paths := ExpectedLayout()
	res := InitResult{Status: "planned", Project: proj, PlannedPaths: append([]string(nil), paths...)}
	if !commit {
		return res, nil
	}

	created := false
	for _, rel := range paths {
		full := filepath.Join(abs, rel)
		if rel == "project.json" {
			if _, err := os.Stat(full); os.IsNotExist(err) {
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					return InitResult{}, err
				}
				raw, err := json.MarshalIndent(proj, "", "  ")
				if err != nil {
					return InitResult{}, err
				}
				if err := os.WriteFile(full, append(raw, '\n'), 0o644); err != nil {
					return InitResult{}, err
				}
				res.ChangedPaths = append(res.ChangedPaths, rel)
				created = true
			}
			continue
		}
		if _, err := os.Stat(full); os.IsNotExist(err) {
			if err := os.MkdirAll(full, 0o755); err != nil {
				return InitResult{}, err
			}
			res.ChangedPaths = append(res.ChangedPaths, rel)
			created = true
		}
	}
	if created {
		res.Status = "created"
	} else {
		res.Status = "no_change"
	}
	return res, nil
}

func Load(path string) (Project, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Project{}, err
	}
	raw, err := os.ReadFile(filepath.Join(abs, "project.json"))
	if err != nil {
		return Project{}, err
	}
	var proj Project
	if err := json.Unmarshal(raw, &proj); err != nil {
		return Project{}, err
	}
	if proj.Root == "" {
		proj.Root = abs
	}
	return proj, nil
}
