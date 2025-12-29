// Package validation provides model validation functionality.
package validation

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
)

// fileExists checks if a file exists in a directory
func fileExists(dir, filename string) bool {
	path := filepath.Join(dir, filename)
	_, err := os.Stat(path)
	return err == nil
}

// validateJSONFile validates that a file contains valid JSON
func validateJSONFile(dir, filename string) (bool, error) {
	path := filepath.Join(dir, filename)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return false, nil
	}

	return true, nil
}

// getDirSize calculates the total size of files in a directory
func getDirSize(dir string) (int64, error) {
	var size int64

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			size += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	return size, nil
}

// listFiles lists all files in a directory
func listFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// readJSONField reads a specific field from a JSON file
func readJSONField(dir, filename, field string) (interface{}, error) {
	path := filepath.Join(dir, filename)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}

	return obj[field], nil
}

// ModelConfig represents the model's config.json structure
type ModelConfig struct {
	ModelType             string `json:"model_type"`
	NumHiddenLayers       int    `json:"num_hidden_layers"`
	HiddenSize            int    `json:"hidden_size"`
	VocabSize             int    `json:"vocab_size"`
	MaxPositionEmbeddings int    `json:"max_position_embeddings"`
}

// readModelConfig reads and parses config.json
func readModelConfig(dir string) (*ModelConfig, error) {
	path := filepath.Join(dir, "config.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config ModelConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// FormatSize formats bytes as human-readable string
func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return formatFloat(float64(bytes)/float64(TB)) + " TB"
	case bytes >= GB:
		return formatFloat(float64(bytes)/float64(GB)) + " GB"
	case bytes >= MB:
		return formatFloat(float64(bytes)/float64(MB)) + " MB"
	case bytes >= KB:
		return formatFloat(float64(bytes)/float64(KB)) + " KB"
	default:
		return formatInt(bytes) + " B"
	}
}

func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return formatInt(int64(f))
	}
	return string([]byte{
		'0' + byte(int(f)/10%10),
		'0' + byte(int(f)%10),
		'.',
		'0' + byte(int(f*10)%10),
		'0' + byte(int(f*100)%10),
	})
}

func formatInt(i int64) string {
	if i == 0 {
		return "0"
	}

	var buf [20]byte
	pos := len(buf)

	for i > 0 {
		pos--
		buf[pos] = '0' + byte(i%10)
		i /= 10
	}

	return string(buf[pos:])
}
