package validation

import (
	"testing"
)

func TestIsVLM(t *testing.T) {
	tests := []struct {
		name     string
		modelName string
		expected  bool
	}{
		{"qwen2-vl lowercase", "qwen2-vl-7b-instruct", true},
		{"qwen2-vl uppercase", "Qwen2-VL-72B", true},
		{"llava", "llava-v1.5-7b", true},
		{"internvl", "internvl-chat-2b", true},
		{"pixtral", "pixtral-12b", true},
		{"phi-3-vision", "phi-3-vision-128k", true},
		{"regular llm 1", "llama-2-7b", false},
		{"regular llm 2", "gpt-4", false},
		{"regular llm 3", "mistral-7b", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsVLM(tt.modelName)
			if result != tt.expected {
				t.Errorf("IsVLM(%q) = %v, expected %v", tt.modelName, result, tt.expected)
			}
		})
	}
}

func TestGetTestImageBase64(t *testing.T) {
	image := GetTestImageBase64()
	
	if len(image) == 0 {
		t.Error("GetTestImageBase64() returned empty string")
	}
	
	// Should be roughly 5100 bytes (base64 encoded 128x128 cat JPEG)
	if len(image) < 5000 || len(image) > 5500 {
		t.Errorf("GetTestImageBase64() length = %d, expected ~5100", len(image))
	}
	
	// Should start with JPEG base64 signature
	if len(image) < 10 {
		t.Fatal("Image too short to check signature")
	}
	
	// JPEG files start with 0xFF 0xD8, which in base64 is "/9j/"
	if image[0:4] != "/9j/" {
		t.Errorf("GetTestImageBase64() doesn't start with JPEG signature, got: %s", image[0:4])
	}
}
