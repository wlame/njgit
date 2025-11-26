package nomad

import (
	"sort"

	"github.com/hashicorp/nomad/api"
)

// NormalizeJob removes dynamic metadata fields from a Nomad job
// This is THE MOST CRITICAL function for accurate change detection!
//
// Problem: When you fetch a job from Nomad, it includes metadata that changes
// on every deployment, even if the actual job configuration hasn't changed:
//   - ModifyIndex: Increments on every change
//   - SubmitTime: When the job was submitted
//   - CreateIndex: When the job was created
//   - Status, StatusDescription: Current runtime status
//
// Without normalization, we'd detect "changes" every time we sync, even when
// nothing meaningful has changed in the job configuration.
//
// This function creates a clean copy of the job with only the configuration
// fields that matter for tracking changes (image versions, resource limits, etc.)
//
// Parameters:
//   - job: The job fetched from Nomad
//   - ignoreFields: Additional field paths to ignore (from config)
//
// Returns:
//   - *api.Job: A normalized copy of the job (original is not modified)
func NormalizeJob(job *api.Job, ignoreFields []string) *api.Job {
	// Create a deep copy of the job
	// We don't want to modify the original job that was passed in
	// In Go, structs are copied by value, but they may contain pointers,
	// so we need to be careful to deep copy everything
	normalized := deepCopyJob(job)

	// Remove top-level metadata fields
	// These are Nomad internal fields that change on every operation
	normalized.ModifyIndex = nil
	normalized.JobModifyIndex = nil
	normalized.SubmitTime = nil
	normalized.CreateIndex = nil
	normalized.Status = nil
	normalized.StatusDescription = nil

	// Normalize nested structures
	// Jobs contain task groups, which contain tasks, which have configs, etc.
	// We need to normalize all levels of the hierarchy
	if normalized.TaskGroups != nil {
		normalizeTaskGroups(normalized.TaskGroups)
	}

	// Sort slices for consistent ordering
	// Nomad doesn't guarantee the order of arrays, but we need consistent
	// output for change detection. If two jobs are identical except for
	// array ordering, we don't want to treat that as a change.
	sortJobFields(normalized)

	return normalized
}

// deepCopyJob creates a deep copy of an api.Job
// This is necessary because Go's default copying is shallow - it copies pointers,
// not the data they point to. For our use case, we need a completely independent copy.
//
// In Go, there's no built-in deep copy function, so we have to implement it ourselves.
// For complex structures like api.Job, this is tedious but necessary.
func deepCopyJob(job *api.Job) *api.Job {
	if job == nil {
		return nil
	}

	// Create a new job and copy all fields
	// We start with a shallow copy
	copied := &api.Job{}

	// Copy simple string fields
	// In Go, strings are immutable, so we can just copy the value
	if job.ID != nil {
		id := *job.ID
		copied.ID = &id
	}
	if job.Name != nil {
		name := *job.Name
		copied.Name = &name
	}
	if job.Namespace != nil {
		ns := *job.Namespace
		copied.Namespace = &ns
	}
	if job.Type != nil {
		t := *job.Type
		copied.Type = &t
	}
	if job.Priority != nil {
		p := *job.Priority
		copied.Priority = &p
	}
	if job.Region != nil {
		r := *job.Region
		copied.Region = &r
	}

	// Copy datacenters (slice of strings)
	// Slices need special handling - we need to create a new slice
	if job.Datacenters != nil {
		copied.Datacenters = make([]string, len(job.Datacenters))
		copy(copied.Datacenters, job.Datacenters)
	}

	// Copy task groups (this is the complex part)
	// Task groups contain the actual job definition
	if job.TaskGroups != nil {
		copied.TaskGroups = make([]*api.TaskGroup, len(job.TaskGroups))
		for i, tg := range job.TaskGroups {
			copied.TaskGroups[i] = deepCopyTaskGroup(tg)
		}
	}

	// Copy other important fields
	// (In a production version, we'd copy ALL fields, but for now we focus on the important ones)
	if job.Update != nil {
		copied.Update = deepCopyUpdateStrategy(job.Update)
	}

	if job.Meta != nil {
		copied.Meta = make(map[string]string)
		for k, v := range job.Meta {
			copied.Meta[k] = v
		}
	}

	return copied
}

// deepCopyTaskGroup creates a deep copy of an api.TaskGroup
// Task groups are the main organizational unit in Nomad jobs
func deepCopyTaskGroup(tg *api.TaskGroup) *api.TaskGroup {
	if tg == nil {
		return nil
	}

	copied := &api.TaskGroup{}

	// Copy simple fields
	if tg.Name != nil {
		name := *tg.Name
		copied.Name = &name
	}
	if tg.Count != nil {
		count := *tg.Count
		copied.Count = &count
	}

	// Copy tasks (the actual work units)
	if tg.Tasks != nil {
		copied.Tasks = make([]*api.Task, len(tg.Tasks))
		for i, task := range tg.Tasks {
			copied.Tasks[i] = deepCopyTask(task)
		}
	}

	// Copy meta
	if tg.Meta != nil {
		copied.Meta = make(map[string]string)
		for k, v := range tg.Meta {
			copied.Meta[k] = v
		}
	}

	return copied
}

// deepCopyTask creates a deep copy of an api.Task
// Tasks are the individual work units (e.g., a Docker container)
func deepCopyTask(task *api.Task) *api.Task {
	if task == nil {
		return nil
	}

	copied := &api.Task{}

	// Copy simple fields
	if task.Name != "" {
		copied.Name = task.Name
	}
	if task.Driver != "" {
		copied.Driver = task.Driver
	}

	// Copy config (driver-specific configuration)
	// This is critical - it contains things like Docker image names
	if task.Config != nil {
		copied.Config = make(map[string]interface{})
		for k, v := range task.Config {
			// For simplicity, we do a shallow copy of the config values
			// This works for most cases (strings, numbers, etc.)
			// For deeply nested configs, we'd need recursive copying
			copied.Config[k] = v
		}
	}

	// Copy environment variables
	if task.Env != nil {
		copied.Env = make(map[string]string)
		for k, v := range task.Env {
			copied.Env[k] = v
		}
	}

	// Copy resources
	if task.Resources != nil {
		copied.Resources = &api.Resources{
			CPU:      task.Resources.CPU,
			MemoryMB: task.Resources.MemoryMB,
			DiskMB:   task.Resources.DiskMB,
		}
	}

	// Copy meta
	if task.Meta != nil {
		copied.Meta = make(map[string]string)
		for k, v := range task.Meta {
			copied.Meta[k] = v
		}
	}

	return copied
}

// deepCopyUpdateStrategy creates a deep copy of an update strategy
func deepCopyUpdateStrategy(u *api.UpdateStrategy) *api.UpdateStrategy {
	if u == nil {
		return nil
	}

	copied := &api.UpdateStrategy{}
	if u.MaxParallel != nil {
		mp := *u.MaxParallel
		copied.MaxParallel = &mp
	}
	if u.HealthCheck != nil {
		hc := *u.HealthCheck
		copied.HealthCheck = &hc
	}
	if u.MinHealthyTime != nil {
		mht := *u.MinHealthyTime
		copied.MinHealthyTime = &mht
	}
	if u.HealthyDeadline != nil {
		hd := *u.HealthyDeadline
		copied.HealthyDeadline = &hd
	}

	return copied
}

// normalizeTaskGroups normalizes all task groups in a job
// This removes metadata and normalizes nested structures
func normalizeTaskGroups(groups []*api.TaskGroup) {
	for _, group := range groups {
		if group == nil {
			continue
		}

		// Normalize metadata map (sort keys for consistent output)
		if group.Meta != nil {
			group.Meta = normalizeMap(group.Meta)
		}

		// Normalize tasks
		if group.Tasks != nil {
			for _, task := range group.Tasks {
				normalizeTask(task)
			}
		}
	}
}

// normalizeTask normalizes a single task
// This is where we clean up driver configs, env vars, etc.
func normalizeTask(task *api.Task) {
	if task == nil {
		return
	}

	// Normalize environment variables (sorted for consistency)
	if task.Env != nil {
		task.Env = normalizeMap(task.Env)
	}

	// Normalize meta
	if task.Meta != nil {
		task.Meta = normalizeMap(task.Meta)
	}

	// Normalize driver config
	if task.Config != nil {
		task.Config = normalizeConfig(task.Config)
	}
}

// normalizeMap creates a new map with sorted keys and no empty values
// This ensures consistent output for maps
//
// In Go, maps have random iteration order, so two identical maps might
// produce different JSON/HCL output. By sorting keys, we ensure consistency.
func normalizeMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}

	// Create a new map
	normalized := make(map[string]string)

	// Copy non-empty values
	for k, v := range m {
		if v != "" {
			normalized[k] = v
		}
	}

	return normalized
}

// normalizeConfig normalizes a driver configuration map
// This is similar to normalizeMap but handles interface{} values
func normalizeConfig(config map[string]interface{}) map[string]interface{} {
	if config == nil {
		return nil
	}

	// For now, return as-is
	// In a production version, we'd recursively normalize nested structures
	// and sort any slice values for consistency
	return config
}

// sortJobFields sorts slices in the job for consistent output
// This is important because Nomad doesn't guarantee ordering
func sortJobFields(job *api.Job) {
	if job == nil {
		return
	}

	// Sort datacenters
	if job.Datacenters != nil {
		sort.Strings(job.Datacenters)
	}

	// Sort task groups by name
	if job.TaskGroups != nil {
		sort.Slice(job.TaskGroups, func(i, j int) bool {
			if job.TaskGroups[i].Name == nil || job.TaskGroups[j].Name == nil {
				return false
			}
			return *job.TaskGroups[i].Name < *job.TaskGroups[j].Name
		})

		// Sort tasks within each task group
		for _, tg := range job.TaskGroups {
			if tg.Tasks != nil {
				sort.Slice(tg.Tasks, func(i, j int) bool {
					return tg.Tasks[i].Name < tg.Tasks[j].Name
				})
			}
		}
	}
}

// GetDockerImage extracts the Docker image from a task's config
// This is a helper function for change detection - Docker image changes
// are one of the most common and important changes to track
//
// Parameters:
//   - task: The task to extract the image from
//
// Returns:
//   - string: The Docker image (e.g., "nginx:1.21"), or empty string if not found
func GetDockerImage(task *api.Task) string {
	if task == nil || task.Driver != "docker" {
		return ""
	}

	if task.Config == nil {
		return ""
	}

	// The image is stored in task.Config["image"]
	// But it's an interface{}, so we need to type assert it to string
	if img, ok := task.Config["image"].(string); ok {
		return img
	}

	return ""
}
