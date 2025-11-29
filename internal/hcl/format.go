// Package hcl handles converting Nomad jobs to and from HCL format.
// HCL (HashiCorp Configuration Language) is the native format for Nomad job files.
package hcl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
)

// FormatJobAsHCL converts a Nomad job to HCL format
// This is a simplified implementation that handles the most common job fields.
// A full implementation would handle all possible Nomad job fields, but this
// is sufficient for most use cases.
//
// We're using a simple string-based approach rather than the hclwrite package
// because the Nomad API doesn't directly map to HCL structures. This gives us
// more control over the output format.
//
// Parameters:
//   - job: The Nomad job to convert (should be normalized first)
//
// Returns:
//   - []byte: The HCL representation of the job
//   - error: Any error encountered during formatting
func FormatJobAsHCL(job *api.Job) ([]byte, error) {
	if job == nil {
		return nil, fmt.Errorf("job cannot be nil")
	}

	// Validate required fields
	if job.ID == nil || *job.ID == "" {
		return nil, fmt.Errorf("job ID is required")
	}

	// Build HCL as a string
	// We use a strings.Builder for efficient string concatenation
	var b strings.Builder

	// Write the job block
	// HCL syntax: job "name" { ... }
	b.WriteString(fmt.Sprintf("job \"%s\" {\n", stringValue(job.ID)))

	// Write job-level attributes in a consistent order
	// Order matters for consistent output (needed for change detection)
	writeJobAttributes(&b, job)

	// Write task groups
	if len(job.TaskGroups) > 0 {
		for _, tg := range job.TaskGroups {
			writeTaskGroup(&b, tg)
		}
	}

	// Close the job block
	b.WriteString("}\n")

	return []byte(b.String()), nil
}

// writeJobAttributes writes job-level attributes to the HCL output
// These are the top-level job settings like namespace, datacenters, type, etc.
func writeJobAttributes(b *strings.Builder, job *api.Job) {
	// Namespace
	if job.Namespace != nil && *job.Namespace != "" && *job.Namespace != "default" {
		// Only write namespace if it's not the default
		writeAttribute(b, 1, "namespace", stringValue(job.Namespace))
	}

	// Datacenters (list of strings)
	if len(job.Datacenters) > 0 {
		writeListAttribute(b, 1, "datacenters", job.Datacenters)
	}

	// Type (service, batch, system)
	if job.Type != nil && *job.Type != "" {
		writeAttribute(b, 1, "type", stringValue(job.Type))
	}

	// Priority
	if job.Priority != nil {
		writeIntAttribute(b, 1, "priority", *job.Priority)
	}

	// Region
	if job.Region != nil && *job.Region != "" {
		writeAttribute(b, 1, "region", stringValue(job.Region))
	}

	// Meta (key-value pairs)
	if len(job.Meta) > 0 {
		writeMapBlock(b, 1, "meta", job.Meta)
	}

	// Update strategy
	if job.Update != nil {
		writeUpdateBlock(b, 1, job.Update)
	}

	// Add a blank line after attributes for readability
	b.WriteString("\n")
}

// writeTaskGroup writes a task group block to the HCL output
// Task groups are the main organizational unit in Nomad jobs
func writeTaskGroup(b *strings.Builder, tg *api.TaskGroup) {
	if tg == nil || tg.Name == nil {
		return
	}

	// Write task group block
	// Indentation level 1 (inside job block)
	indent(b, 1)
	fmt.Fprintf(b, "group \"%s\" {\n", stringValue(tg.Name))

	// Count (number of instances)
	if tg.Count != nil {
		writeIntAttribute(b, 2, "count", *tg.Count)
	}

	// Meta
	if len(tg.Meta) > 0 {
		writeMapBlock(b, 2, "meta", tg.Meta)
	}

	// Write tasks
	if len(tg.Tasks) > 0 {
		b.WriteString("\n")
		for _, task := range tg.Tasks {
			writeTask(b, task)
		}
	}

	// Close task group block
	indent(b, 1)
	b.WriteString("}\n\n")
}

// writeTask writes a task block to the HCL output
// Tasks are the actual work units (e.g., Docker containers)
func writeTask(b *strings.Builder, task *api.Task) {
	if task == nil || task.Name == "" {
		return
	}

	// Write task block
	// Indentation level 2 (inside task group)
	indent(b, 2)
	fmt.Fprintf(b, "task \"%s\" {\n", task.Name)

	// Driver (docker, exec, java, etc.)
	if task.Driver != "" {
		writeAttribute(b, 3, "driver", task.Driver)
	}

	// Config (driver-specific configuration)
	if len(task.Config) > 0 {
		writeConfigBlock(b, 3, task.Config)
	}

	// Environment variables
	if len(task.Env) > 0 {
		writeMapBlock(b, 3, "env", task.Env)
	}

	// Resources
	if task.Resources != nil {
		writeResourcesBlock(b, 3, task.Resources)
	}

	// Meta
	if len(task.Meta) > 0 {
		writeMapBlock(b, 3, "meta", task.Meta)
	}

	// Close task block
	indent(b, 2)
	b.WriteString("}\n")
}

// writeUpdateBlock writes an update strategy block
func writeUpdateBlock(b *strings.Builder, level int, update *api.UpdateStrategy) {
	if update == nil {
		return
	}

	indent(b, level)
	b.WriteString("update {\n")

	if update.MaxParallel != nil && *update.MaxParallel > 0 {
		writeIntAttribute(b, level+1, "max_parallel", *update.MaxParallel)
	}

	if update.HealthCheck != nil && *update.HealthCheck != "" {
		writeAttribute(b, level+1, "health_check", *update.HealthCheck)
	}

	indent(b, level)
	b.WriteString("}\n")
}

// writeResourcesBlock writes a resources block
func writeResourcesBlock(b *strings.Builder, level int, resources *api.Resources) {
	if resources == nil {
		return
	}

	indent(b, level)
	b.WriteString("resources {\n")

	if resources.CPU != nil && *resources.CPU > 0 {
		writeIntAttribute(b, level+1, "cpu", *resources.CPU)
	}

	if resources.MemoryMB != nil && *resources.MemoryMB > 0 {
		writeIntAttribute(b, level+1, "memory", *resources.MemoryMB)
	}

	indent(b, level)
	b.WriteString("}\n")
}

// writeConfigBlock writes a config block with driver-specific settings
func writeConfigBlock(b *strings.Builder, level int, config map[string]interface{}) {
	if len(config) == 0 {
		return
	}

	indent(b, level)
	b.WriteString("config {\n")

	// Sort keys for consistent output
	keys := make([]string, 0, len(config))
	for k := range config {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Write each config value
	for _, key := range keys {
		value := config[key]
		writeConfigValue(b, level+1, key, value)
	}

	indent(b, level)
	b.WriteString("}\n")
}

// writeConfigValue writes a single config key-value pair
// This handles different value types (string, int, bool, list)
func writeConfigValue(b *strings.Builder, level int, key string, value interface{}) {
	indent(b, level)

	switch v := value.(type) {
	case string:
		fmt.Fprintf(b, "%s = \"%s\"\n", key, escapeString(v))
	case int:
		fmt.Fprintf(b, "%s = %d\n", key, v)
	case int64:
		fmt.Fprintf(b, "%s = %d\n", key, v)
	case float64:
		fmt.Fprintf(b, "%s = %g\n", key, v)
	case bool:
		fmt.Fprintf(b, "%s = %t\n", key, v)
	case []interface{}:
		// List of values
		fmt.Fprintf(b, "%s = [", key)
		for i, item := range v {
			if i > 0 {
				b.WriteString(", ")
			}
			if s, ok := item.(string); ok {
				fmt.Fprintf(b, "\"%s\"", escapeString(s))
			} else {
				fmt.Fprintf(b, "%v", item)
			}
		}
		b.WriteString("]\n")
	default:
		// For other types, use default string representation
		fmt.Fprintf(b, "%s = %v\n", key, v)
	}
}

// writeMapBlock writes a map block (for meta, env, etc.)
func writeMapBlock(b *strings.Builder, level int, name string, m map[string]string) {
	if len(m) == 0 {
		return
	}

	indent(b, level)
	fmt.Fprintf(b, "%s {\n", name)

	// Sort keys for consistent output
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Write each key-value pair
	for _, key := range keys {
		indent(b, level+1)
		fmt.Fprintf(b, "%s = \"%s\"\n", key, escapeString(m[key]))
	}

	indent(b, level)
	b.WriteString("}\n")
}

// writeAttribute writes a simple string attribute
func writeAttribute(b *strings.Builder, level int, name, value string) {
	if value == "" {
		return
	}
	indent(b, level)
	fmt.Fprintf(b, "%s = \"%s\"\n", name, escapeString(value))
}

// writeIntAttribute writes an integer attribute
func writeIntAttribute(b *strings.Builder, level int, name string, value int) {
	indent(b, level)
	fmt.Fprintf(b, "%s = %d\n", name, value)
}

// writeListAttribute writes a list of strings
func writeListAttribute(b *strings.Builder, level int, name string, values []string) {
	if len(values) == 0 {
		return
	}

	indent(b, level)
	fmt.Fprintf(b, "%s = [", name)

	for i, v := range values {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(b, "\"%s\"", escapeString(v))
	}

	b.WriteString("]\n")
}

// indent writes the appropriate indentation for the given level
// Each level is 2 spaces
func indent(b *strings.Builder, level int) {
	for i := 0; i < level; i++ {
		b.WriteString("  ")
	}
}

// stringValue safely extracts a string from a *string
// Returns empty string if the pointer is nil
func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// escapeString escapes special characters in strings for HCL
// This handles quotes, newlines, and other special characters
func escapeString(s string) string {
	// Replace backslashes first
	s = strings.ReplaceAll(s, "\\", "\\\\")
	// Replace quotes
	s = strings.ReplaceAll(s, "\"", "\\\"")
	// Replace newlines
	s = strings.ReplaceAll(s, "\n", "\\n")
	// Replace tabs
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// ParseHCL parses HCL content and returns a Nomad Job struct
// This uses the Nomad API client to parse HCL
//
// Parameters:
//   - hclContent: The HCL content as bytes
//
// Returns:
//   - *api.Job: The parsed job
//   - error: Any error encountered during parsing
func ParseHCL(hclContent []byte, nomadAddr ...string) (*api.Job, error) {
	if len(hclContent) == 0 {
		return nil, fmt.Errorf("HCL content is empty")
	}

	// Create a Nomad client configuration
	config := api.DefaultConfig()

	// If a Nomad address is provided, use it
	if len(nomadAddr) > 0 && nomadAddr[0] != "" {
		config.Address = nomadAddr[0]
	}

	// Disable TLS verification for testing
	config.TLSConfig = &api.TLSConfig{
		Insecure: true,
	}

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	// Use the client's Jobs().ParseHCL method
	// Note: This makes a request to Nomad's /v1/jobs/parse endpoint
	// The canonicalize parameter (false) means we don't process the job further
	job, err := client.Jobs().ParseHCL(string(hclContent), false)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HCL: %w", err)
	}

	return job, nil
}
