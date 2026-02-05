package workflow

import (
	"sync"
	"time"
)

// ResourceLock manages file-level locks to prevent conflicts
type ResourceLock struct {
	mu    sync.Mutex
	locks map[string]*lockInfo
}

type lockInfo struct {
	holder  string    // task ID
	created time.Time
}

// NewResourceLock creates a new resource lock manager
func NewResourceLock() *ResourceLock {
	return &ResourceLock{
		locks: make(map[string]*lockInfo),
	}
}

// TryLock attempts to acquire locks for all resources
// Returns true if all locks were acquired, false if any resource is already locked
func (rl *ResourceLock) TryLock(taskID string, resources []string) bool {
	if len(resources) == 0 {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Check if any resource is already locked by another task
	for _, res := range resources {
		if info, exists := rl.locks[res]; exists && info.holder != taskID {
			return false
		}
	}

	// Acquire all locks
	now := time.Now()
	for _, res := range resources {
		rl.locks[res] = &lockInfo{
			holder:  taskID,
			created: now,
		}
	}

	return true
}

// Unlock releases all locks held by a task
func (rl *ResourceLock) Unlock(taskID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for res, info := range rl.locks {
		if info.holder == taskID {
			delete(rl.locks, res)
		}
	}
}

// UnlockResource releases a specific resource lock
func (rl *ResourceLock) UnlockResource(taskID, resource string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if info, exists := rl.locks[resource]; exists && info.holder == taskID {
		delete(rl.locks, resource)
		return true
	}
	return false
}

// IsLocked checks if a resource is locked
func (rl *ResourceLock) IsLocked(resource string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	_, exists := rl.locks[resource]
	return exists
}

// GetHolder returns the task ID holding the lock on a resource
func (rl *ResourceLock) GetHolder(resource string) (string, bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if info, exists := rl.locks[resource]; exists {
		return info.holder, true
	}
	return "", false
}

// GetLockedResources returns all resources locked by a task
func (rl *ResourceLock) GetLockedResources(taskID string) []string {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	var resources []string
	for res, info := range rl.locks {
		if info.holder == taskID {
			resources = append(resources, res)
		}
	}
	return resources
}

// GetAllLocks returns a copy of all current locks
func (rl *ResourceLock) GetAllLocks() map[string]string {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	result := make(map[string]string)
	for res, info := range rl.locks {
		result[res] = info.holder
	}
	return result
}

// Clear removes all locks
func (rl *ResourceLock) Clear() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.locks = make(map[string]*lockInfo)
}
