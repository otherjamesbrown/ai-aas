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
	filenamePattern = regexp.MustCompile(`^[0-9]{12}_[a-z0-9_]+\.(up|down)\.sql$`)
	versionPattern  = regexp.MustCompile(`^([0-9]{12})_([a-z0-9_]+)\.(up|down)\.sql$`)
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
	components := []string{"operational", "analytics"}
	var issues []string

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
		seenVersions := map[string]map[string]bool{}

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
				issues = append(issues, fmt.Sprintf("%s: filename must match <YYYYMMDDHHMM>_<slug>.up|down.sql", filepath.Join(component, name)))
				return nil
			}

			matches := versionPattern.FindStringSubmatch(name)
			if matches == nil {
				return nil
			}

			version := matches[1]
			direction := matches[3]
			if seenVersions[version] == nil {
				seenVersions[version] = map[string]bool{}
			}
			if seenVersions[version][direction] {
				issues = append(issues, fmt.Sprintf("duplicate %s migration for version %s in %s", direction, version, component))
			}
			seenVersions[version][direction] = true
			return nil
		})
		if err != nil {
			return nil, err
		}

		for version, dirs := range seenVersions {
			if !(dirs["up"] && dirs["down"]) {
				issues = append(issues, fmt.Sprintf("missing up/down pair for version %s in %s", version, component))
			}
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

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			issues = append(issues, fmt.Sprintf("classification file missing: %s", path))
			return issues, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
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
