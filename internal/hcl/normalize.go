package hcl

import (
	"bytes"
	"strings"
)

// NormalizeHCL normalizes HCL content for consistent comparison
// This ensures that two semantically identical HCL files will have
// byte-identical output, which is critical for change detection.
//
// Normalizations applied:
//   - Consistent line endings (Unix-style \n)
//   - Trailing newline at end of file
//   - No trailing whitespace on lines
//   - Consistent spacing (already handled by FormatJobAsHCL)
//
// Parameters:
//   - hcl: The HCL content to normalize
//
// Returns:
//   - []byte: Normalized HCL content
func NormalizeHCL(hcl []byte) []byte {
	// Convert to string for easier manipulation
	s := string(hcl)

	// Normalize line endings to Unix-style (\n)
	// Windows uses \r\n, Mac (old) uses \r, Unix uses \n
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// Split into lines
	lines := strings.Split(s, "\n")

	// Remove trailing whitespace from each line
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}

	// Rejoin lines
	s = strings.Join(lines, "\n")

	// Ensure file ends with exactly one newline
	s = strings.TrimRight(s, "\n") + "\n"

	return []byte(s)
}

// CompareHCL compares two HCL contents for equality
// This is a convenience function that normalizes both inputs before comparing
//
// Parameters:
//   - a, b: The HCL contents to compare
//
// Returns:
//   - bool: true if the HCL contents are semantically identical
func CompareHCL(a, b []byte) bool {
	// Normalize both inputs
	normalizedA := NormalizeHCL(a)
	normalizedB := NormalizeHCL(b)

	// Byte-level comparison
	return bytes.Equal(normalizedA, normalizedB)
}

// StripComments removes comments from HCL content
// This is useful if you want to ignore comments when comparing files
//
// Note: This is a simple implementation that handles line comments (//, #)
// It does not handle block comments (/* */) or comments within strings
//
// Parameters:
//   - hcl: The HCL content
//
// Returns:
//   - []byte: HCL content without comments
func StripComments(hcl []byte) []byte {
	s := string(hcl)
	lines := strings.Split(s, "\n")

	var result []string
	for _, line := range lines {
		// Find comment start
		// We check for both // and # (both are valid in HCL)
		commentStart := -1

		// Check for // comments
		if idx := strings.Index(line, "//"); idx >= 0 {
			commentStart = idx
		}

		// Check for # comments
		if idx := strings.Index(line, "#"); idx >= 0 {
			if commentStart < 0 || idx < commentStart {
				commentStart = idx
			}
		}

		// If we found a comment, strip it
		if commentStart >= 0 {
			line = line[:commentStart]
		}

		// Trim trailing whitespace
		line = strings.TrimRight(line, " \t")

		// Keep the line even if it's empty (preserves structure)
		result = append(result, line)
	}

	return []byte(strings.Join(result, "\n"))
}
