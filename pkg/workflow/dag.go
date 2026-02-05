package workflow

import (
	"fmt"
	"sort"
)

// DAG represents a directed acyclic graph for task dependencies
type DAG struct {
	nodes   map[string]*Task
	edges   map[string][]string // task ID -> depends on task IDs
	reverse map[string][]string // task ID -> depended by task IDs
}

// NewDAG creates a new DAG from tasks
func NewDAG(tasks []*Task) *DAG {
	dag := &DAG{
		nodes:   make(map[string]*Task),
		edges:   make(map[string][]string),
		reverse: make(map[string][]string),
	}

	for _, task := range tasks {
		dag.nodes[task.ID] = task
		dag.edges[task.ID] = task.DependsOn

		for _, depID := range task.DependsOn {
			dag.reverse[depID] = append(dag.reverse[depID], task.ID)
		}
	}

	return dag
}

// GetTask returns a task by ID
func (d *DAG) GetTask(id string) (*Task, bool) {
	task, ok := d.nodes[id]
	return task, ok
}

// GetAllTasks returns all tasks
func (d *DAG) GetAllTasks() []*Task {
	tasks := make([]*Task, 0, len(d.nodes))
	for _, task := range d.nodes {
		tasks = append(tasks, task)
	}
	return tasks
}

// GetDependencies returns the dependencies of a task
func (d *DAG) GetDependencies(taskID string) []string {
	return d.edges[taskID]
}

// GetDependents returns tasks that depend on the given task
func (d *DAG) GetDependents(taskID string) []string {
	return d.reverse[taskID]
}

// GetReadyTasks returns tasks with all dependencies satisfied
func (d *DAG) GetReadyTasks(completed map[string]bool) []*Task {
	var ready []*Task

	for id, task := range d.nodes {
		// Skip already completed or running tasks
		if completed[id] || task.Status == TaskStatusRunning ||
			task.Status == TaskStatusCompleted || task.Status == TaskStatusCancelled {
			continue
		}

		// Check if all dependencies are completed
		allDepsCompleted := true
		for _, depID := range d.edges[id] {
			if !completed[depID] {
				allDepsCompleted = false
				break
			}
		}

		if allDepsCompleted {
			ready = append(ready, task)
		}
	}

	// Sort by priority (lower number = higher priority)
	sort.Slice(ready, func(i, j int) bool {
		return ready[i].Priority < ready[j].Priority
	})

	return ready
}

// TopologicalSort returns tasks in execution order using Kahn's algorithm
func (d *DAG) TopologicalSort() ([]*Task, error) {
	// Calculate in-degree for each node
	inDegree := make(map[string]int)
	for id := range d.nodes {
		inDegree[id] = len(d.edges[id])
	}

	// Find all nodes with no dependencies
	var queue []*Task
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, d.nodes[id])
		}
	}

	// Sort initial queue by priority
	sort.Slice(queue, func(i, j int) bool {
		return queue[i].Priority < queue[j].Priority
	})

	var result []*Task
	for len(queue) > 0 {
		// Pop first task
		task := queue[0]
		queue = queue[1:]
		result = append(result, task)

		// Reduce in-degree of dependent tasks
		for _, childID := range d.reverse[task.ID] {
			inDegree[childID]--
			if inDegree[childID] == 0 {
				queue = append(queue, d.nodes[childID])
			}
		}

		// Re-sort queue by priority
		sort.Slice(queue, func(i, j int) bool {
			return queue[i].Priority < queue[j].Priority
		})
	}

	// Check for circular dependency
	if len(result) != len(d.nodes) {
		return nil, fmt.Errorf("circular dependency detected: processed %d of %d tasks", len(result), len(d.nodes))
	}

	return result, nil
}

// Validate checks if the DAG is valid (no circular dependencies, all deps exist)
func (d *DAG) Validate() error {
	// Check all dependencies exist
	for id, deps := range d.edges {
		for _, depID := range deps {
			if _, exists := d.nodes[depID]; !exists {
				return fmt.Errorf("task %s depends on non-existent task %s", id, depID)
			}
		}
	}

	// Check for circular dependencies via topological sort
	_, err := d.TopologicalSort()
	return err
}

// GetExecutionLevels returns tasks grouped by execution level
// Tasks in the same level can be executed concurrently
func (d *DAG) GetExecutionLevels() ([][]* Task, error) {
	sorted, err := d.TopologicalSort()
	if err != nil {
		return nil, err
	}

	// Calculate level for each task
	levels := make(map[string]int)
	for _, task := range sorted {
		maxDepLevel := -1
		for _, depID := range d.edges[task.ID] {
			if levels[depID] > maxDepLevel {
				maxDepLevel = levels[depID]
			}
		}
		levels[task.ID] = maxDepLevel + 1
	}

	// Group tasks by level
	maxLevel := 0
	for _, level := range levels {
		if level > maxLevel {
			maxLevel = level
		}
	}

	result := make([][]*Task, maxLevel+1)
	for _, task := range sorted {
		level := levels[task.ID]
		result[level] = append(result[level], task)
	}

	return result, nil
}
