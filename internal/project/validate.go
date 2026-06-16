package project

import (
	"fmt"
	"regexp"
	"strings"
)

var projectIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func Validate(proj Project) error {
	if proj.Version != "vflow-project/v1" {
		return fmt.Errorf("project version must be vflow-project/v1")
	}
	if strings.TrimSpace(proj.ID) == "" {
		return fmt.Errorf("project id is required")
	}
	if !projectIDPattern.MatchString(proj.ID) {
		return fmt.Errorf("project id must start with a letter or number and contain only letters, numbers, underscore, dash, or dot")
	}
	if strings.TrimSpace(proj.Root) == "" {
		return fmt.Errorf("project root is required")
	}
	if proj.CreatedAt.IsZero() {
		return fmt.Errorf("project created_at is required")
	}
	if proj.UpdatedAt.IsZero() {
		return fmt.Errorf("project updated_at is required")
	}
	if proj.UpdatedAt.Before(proj.CreatedAt) {
		return fmt.Errorf("project updated_at must be at or after created_at")
	}
	return nil
}
