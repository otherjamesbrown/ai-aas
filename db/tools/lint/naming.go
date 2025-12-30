package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	// Pattern for single-file goose migrations: YYYYMMDDHHMM_description.sql
	filenamePattern = regexp.MustCompile(`^[0-9]{11,12}_[a-z0-9_]+\.sql$`)
	versionPattern  = regexp.MustCompile(`^([0-9]{11,12})_([a-z0-9_]+)\.sql$`)
)

type classificationConfig struct {
	Entities []struct {
		Name   string `yaml:"name"`
		Fields []struct {
			Name               string `yaml:"name"`
			Classification     string `yaml:"classification"`
			EncryptionRequired bool   `yaml:"encryption_required"`
			HashRequired       bool   `yaml:"hash_required"`
		} `yaml:"fields"`
	} `yaml:"entities"`
}

// RunNamingLint returns a slice of human-readable issues if lint violations are detected.
func RunNamingLint(basePath string) ([]string, error) {
	components := []string{"operational", "analytics", "jobs"}
	var issues []string

	// Track versions across all components to detect collisions
	globalVersions := map[string][]string{} // version -> list of components using it

	for _, component := range components {
		dir := filepath.Join(basePath, component)
		files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
		if err != nil {
			return nil, fmt.Errorf("glob %s: %w", dir, err)
		}
		if len(files) == 0 {
			// No migrations yet; nothing to lint.
			continue
		}

		sort.Strings(files)
		seenVersions := map[string]bool{}

		err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if path != dir {
					issues = append(issues, fmt.Sprintf("unexpected subdirectory %s in %s", path, component))
				}
				return nil
			}

			name := d.Name()
			if !filenamePattern.MatchString(name) {
				issues = append(issues, fmt.Sprintf("%s: filename must match YYYYMMDDHHMM_<slug>.sql", filepath.Join(component, name)))
				return nil
			}

			matches := versionPattern.FindStringSubmatch(name)
			if matches == nil {
				return nil
			}

			version := matches[1]
			if seenVersions[version] {
				issues = append(issues, fmt.Sprintf("duplicate migration version %s in %s", version, component))
				return nil
			}
			seenVersions[version] = true
			// Track this version globally
			globalVersions[version] = append(globalVersions[version], component)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// Check for version collisions across components
	for version, components := range globalVersions {
		if len(components) > 1 {
			sort.Strings(components) // Ensure consistent error message ordering
			files := []string{}
			for _, component := range components {
				// Find the actual filename for better error message
				dir := filepath.Join(basePath, component)
				matches, _ := filepath.Glob(filepath.Join(dir, version+"_*.sql"))
				if len(matches) > 0 {
					files = append(files, matches[0])
				}
			}
			issues = append(issues, fmt.Sprintf("migration version collision: version %s used in multiple components: %s\nConflicting files:\n  - %s\n\nMigration versions must be unique across all components to avoid goose_db_version conflicts.",
				version,
				strings.Join(components, ", "),
				strings.Join(files, "\n  - ")))
		}
	}

	if classificationIssues, err := validateClassification("configs/data-classification.yml"); err != nil {
		return nil, err
	} else {
		issues = append(issues, classificationIssues...)
	}

	return issues, nil
}

func validateClassification(path string) ([]string, error) {
	var issues []string

	candidates := []string{
		path,
		filepath.Join("..", "..", "..", path),
	}

	var content []byte
	var err error
	for _, candidate := range candidates {
		content, err = os.ReadFile(candidate)
		if err == nil {
			path = candidate
			break
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read %s: %w", candidate, err)
		}
	}
	if err != nil {
		issues = append(issues, fmt.Sprintf("classification file missing: %s", path))
		return issues, nil
	}

	var cfg classificationConfig
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	for _, entity := range cfg.Entities {
		for _, field := range entity.Fields {
			if strings.EqualFold(field.Classification, "restricted") &&
				!(field.EncryptionRequired || field.HashRequired) {
				issues = append(issues, fmt.Sprintf("classification: %s.%s marked restricted but missing encryption or hash requirement", entity.Name, field.Name))
			}
		}
	}

	return issues, nil
}
