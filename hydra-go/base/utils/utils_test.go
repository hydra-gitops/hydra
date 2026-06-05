package utils

import (
	"os"
	"testing"
)

func TestClone(t *testing.T) {
	t.Run("non-nil pointer", func(t *testing.T) {
		value := 42
		ptr := &value
		cloned := Clone(ptr)

		if cloned == nil {
			t.Fatal("Clone returned nil for non-nil input")
		}
		if cloned == ptr {
			t.Error("Clone should return a different pointer")
		}
		if *cloned != *ptr {
			t.Errorf("expected %d, got %d", *ptr, *cloned)
		}
	})

	t.Run("nil pointer", func(t *testing.T) {
		var ptr *int = nil
		cloned := Clone(ptr)

		if cloned != nil {
			t.Error("Clone should return nil for nil input")
		}
	})

	t.Run("modification independence", func(t *testing.T) {
		value := 42
		ptr := &value
		cloned := Clone(ptr)

		// Modify original
		*ptr = 100

		if *cloned != 42 {
			t.Errorf("cloned value should not be affected, expected 42, got %d", *cloned)
		}
	})

	t.Run("struct", func(t *testing.T) {
		type testStruct struct {
			Name string
			Age  int
		}
		value := testStruct{Name: "test", Age: 25}
		ptr := &value
		cloned := Clone(ptr)

		if cloned == ptr {
			t.Error("Clone should return a different pointer")
		}
		if *cloned != *ptr {
			t.Errorf("expected %+v, got %+v", *ptr, *cloned)
		}
	})
}

func TestFileUriToPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with file prefix",
			input:    "file:///home/user/file.txt",
			expected: "/home/user/file.txt",
		},
		{
			name:     "without file prefix",
			input:    "/home/user/file.txt",
			expected: "/home/user/file.txt",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only file prefix",
			input:    "file://",
			expected: "",
		},
		{
			name:     "windows style path",
			input:    "file:///C:/Users/file.txt",
			expected: "/C:/Users/file.txt",
		},
		{
			name:     "relative path",
			input:    "file://./relative/path",
			expected: "./relative/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FileUriToPath(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestEnvWrapper(t *testing.T) {
	const testEnvVar = "HYDRA_TEST_ENV_VAR"

	t.Run("sets and restores environment variable", func(t *testing.T) {
		// Set initial value
		os.Setenv(testEnvVar, "original")
		defer os.Unsetenv(testEnvVar)

		// Use EnvWrapper
		restore := EnvWrapper(testEnvVar, "temporary")

		// Check temporary value is set
		if got := os.Getenv(testEnvVar); got != "temporary" {
			t.Errorf("expected 'temporary', got %q", got)
		}

		// Restore
		restore()

		// Check original value is restored
		if got := os.Getenv(testEnvVar); got != "original" {
			t.Errorf("expected 'original', got %q", got)
		}
	})

	t.Run("restores empty value", func(t *testing.T) {
		// Ensure variable is not set
		os.Unsetenv(testEnvVar)

		restore := EnvWrapper(testEnvVar, "temporary")

		if got := os.Getenv(testEnvVar); got != "temporary" {
			t.Errorf("expected 'temporary', got %q", got)
		}

		restore()

		// Should be empty after restore
		if got := os.Getenv(testEnvVar); got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("multiple restores are idempotent", func(t *testing.T) {
		os.Setenv(testEnvVar, "original")
		defer os.Unsetenv(testEnvVar)

		restore := EnvWrapper(testEnvVar, "temporary")

		restore()
		restore() // Second call should be safe

		if got := os.Getenv(testEnvVar); got != "original" {
			t.Errorf("expected 'original', got %q", got)
		}
	})
}
