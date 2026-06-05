package sops

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// DecryptSopsFile decrypts a SOPS-encrypted file and returns the plaintext content.
func DecryptSopsFile(path string) (types.YamlString, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// sops reads from absPath and writes decrypted content to stdout.
	cmd := exec.Command("sops", "--decrypt", absPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to decrypt file %s: %w\nstderr: %s", absPath, err, stderr.String())
	}

	return types.YamlString(stdout.String()), nil
}

// DecryptSopsYaml decrypts SOPS-encrypted YAML content in memory via stdin.
func DecryptSopsYaml(data types.YamlString) (types.YamlString, error) {
	cmd := exec.Command("sops", "--decrypt", "--input-type", "yaml", "--output-type", "yaml", "/dev/stdin")
	cmd.Stdin = bytes.NewReader([]byte(data))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to decrypt YAML via stdin: %w\nstderr: %s", err, stderr.String())
	}

	return types.YamlString(stdout.String()), nil
}

// EncryptSopsYaml encrypts YAML content in memory and returns the encrypted
// document as YAML. The target path is still provided to sops via
// --filename-override so matching creation_rules apply.
func EncryptSopsYaml(data types.YamlString, path string) (types.YamlString, error) {
	return EncryptSopsYamlWithConfig(data, path, "")
}

// EncryptSopsYamlWithConfig encrypts YAML content in memory and optionally
// passes an explicit SOPS config file via --config.
func EncryptSopsYamlWithConfig(data types.YamlString, path string, configPath string) (types.YamlString, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	filenameOverride := absPath
	args := []string{"--encrypt"}
	if configPath != "" {
		absConfigPath, err := filepath.Abs(configPath)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute config path: %w", err)
		}
		relPath, err := filepath.Rel(filepath.Dir(absConfigPath), absPath)
		if err != nil {
			return "", fmt.Errorf("failed to relativize %s against config %s: %w", absPath, absConfigPath, err)
		}
		filenameOverride = filepath.ToSlash(relPath)
		args = append(args, "--config", absConfigPath)
	}
	args = append(args, "--filename-override", filenameOverride)
	args = append(args, "/dev/stdin")
	cmd := exec.Command("sops", args...)
	cmd.Stdin = bytes.NewReader([]byte(data))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to encrypt YAML for %s: %w\nstderr: %s", absPath, err, stderr.String())
	}

	return types.YamlString(stdout.String()), nil
}

// EncryptSopsFile encrypts the data and writes it to the file at path.
// Data is passed via stdin to sops, sops writes the encrypted content to absPath.
func EncryptSopsFile(data types.YamlString, path string) error {
	encrypted, err := EncryptSopsYaml(data, path)
	if err != nil {
		return err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	return os.WriteFile(absPath, []byte(encrypted), 0o644)
}
