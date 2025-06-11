package auth

import (
	"context"
	"fmt"
	"sync"
)

// RBACProvider implements role-based access control
type RBACProvider struct {
	mu            sync.RWMutex
	roles         map[string]*Role
	userRoles     map[string][]string // user ID -> roles
	resourcePerms map[string][]string // resource -> required permissions
	defaultRole   string
}

// Role represents a role with permissions
type Role struct {
	Name        string
	Description string
	Permissions []string
	ParentRoles []string // For role inheritance
}

// NewRBACProvider creates a new RBAC provider
func NewRBACProvider(config *RBACConfig) *RBACProvider {
	provider := &RBACProvider{
		roles:         make(map[string]*Role),
		userRoles:     make(map[string][]string),
		resourcePerms: make(map[string][]string),
		defaultRole:   "user",
	}

	if config != nil {
		provider.defaultRole = config.DefaultRole
		provider.resourcePerms = config.ResourcePermissions

		// Initialize role hierarchy
		for roleName, parentRoles := range config.RoleHierarchy {
			provider.roles[roleName] = &Role{
				Name:        roleName,
				ParentRoles: parentRoles,
			}
		}
	}

	// Initialize default roles
	provider.initializeDefaultRoles()

	return provider
}

// initializeDefaultRoles sets up common roles
func (p *RBACProvider) initializeDefaultRoles() {
	// Admin role with all permissions
	_ = p.CreateRole("admin", "Administrator with full access", []string{"*"}, nil)

	// User role with basic permissions
	_ = p.CreateRole("user", "Standard user", []string{"read", "list", "create", "update"}, nil)

	// Guest role with limited permissions
	_ = p.CreateRole("guest", "Guest user", []string{"read", "list"}, nil)

	// Service role for service accounts
	_ = p.CreateRole("service", "Service account", []string{"read", "write", "execute"}, nil)
}

// CreateRole creates a new role
func (p *RBACProvider) CreateRole(name, description string, permissions []string, parentRoles []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.roles[name]; exists {
		return fmt.Errorf("role %s already exists", name)
	}

	p.roles[name] = &Role{
		Name:        name,
		Description: description,
		Permissions: permissions,
		ParentRoles: parentRoles,
	}

	return nil
}

// DeleteRole removes a role
func (p *RBACProvider) DeleteRole(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if name == p.defaultRole {
		return fmt.Errorf("cannot delete default role")
	}

	delete(p.roles, name)

	// Remove role from all users
	for userID, roles := range p.userRoles {
		p.userRoles[userID] = removeStringFromSlice(roles, name)
	}

	return nil
}

// AssignRole assigns a role to a user
func (p *RBACProvider) AssignRole(userID, roleName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.roles[roleName]; !exists {
		return fmt.Errorf("role %s does not exist", roleName)
	}

	roles := p.userRoles[userID]
	if !stringSliceContains(roles, roleName) {
		p.userRoles[userID] = append(roles, roleName)
	}

	return nil
}

// RemoveRole removes a role from a user
func (p *RBACProvider) RemoveRole(userID, roleName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	roles := p.userRoles[userID]
	p.userRoles[userID] = removeStringFromSlice(roles, roleName)

	return nil
}

// GetUserRoles returns all roles for a user
func (p *RBACProvider) GetUserRoles(userID string) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	roles := p.userRoles[userID]
	if len(roles) == 0 && p.defaultRole != "" {
		return []string{p.defaultRole}
	}

	return append([]string{}, roles...) // Return copy
}

// HasPermission checks if a user has a specific permission
func (p *RBACProvider) HasPermission(userID, permission string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	roles := p.GetUserRoles(userID)
	for _, roleName := range roles {
		if p.roleHasPermission(roleName, permission, make(map[string]bool)) {
			return true
		}
	}

	return false
}

// roleHasPermission checks if a role has a permission (with inheritance)
func (p *RBACProvider) roleHasPermission(roleName, permission string, visited map[string]bool) bool {
	// Prevent infinite recursion
	if visited[roleName] {
		return false
	}
	visited[roleName] = true

	role, exists := p.roles[roleName]
	if !exists {
		return false
	}

	// Check direct permissions
	for _, perm := range role.Permissions {
		if perm == "*" || perm == permission {
			return true
		}
		// Support wildcard matching (e.g., "tools:*" matches "tools:read")
		if matchPermission(perm, permission) {
			return true
		}
	}

	// Check inherited permissions
	for _, parentRole := range role.ParentRoles {
		if p.roleHasPermission(parentRole, permission, visited) {
			return true
		}
	}

	return false
}

// CanAccessResource checks if a user can access a resource
func (p *RBACProvider) CanAccessResource(userID, resource string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Get required permissions for the resource
	requiredPerms, exists := p.resourcePerms[resource]
	if !exists {
		// No specific permissions required
		return true
	}

	// Check if user has any of the required permissions
	for _, perm := range requiredPerms {
		if p.HasPermission(userID, perm) {
			return true
		}
	}

	return false
}

// SetResourcePermissions sets required permissions for a resource
func (p *RBACProvider) SetResourcePermissions(resource string, permissions []string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.resourcePerms[resource] = permissions
}

// GetResourcePermissions returns required permissions for a resource
func (p *RBACProvider) GetResourcePermissions(resource string) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	perms := p.resourcePerms[resource]
	return append([]string{}, perms...) // Return copy
}

// EnforcePermission is a middleware helper that checks permissions
func (p *RBACProvider) EnforcePermission(permission string) func(context.Context) error {
	return func(ctx context.Context) error {
		userInfo, ok := UserInfoFromContext(ctx)
		if !ok {
			return NewAuthError(ErrAuthRequired, "authentication required")
		}

		if !p.HasPermission(userInfo.ID, permission) {
			return NewAuthError(ErrAccessDenied,
				fmt.Sprintf("permission denied: requires %s", permission))
		}

		return nil
	}
}

// EnforceRole is a middleware helper that checks roles
func (p *RBACProvider) EnforceRole(requiredRoles ...string) func(context.Context) error {
	return func(ctx context.Context) error {
		userInfo, ok := UserInfoFromContext(ctx)
		if !ok {
			return NewAuthError(ErrAuthRequired, "authentication required")
		}

		userRoles := p.GetUserRoles(userInfo.ID)
		for _, required := range requiredRoles {
			if stringSliceContains(userRoles, required) {
				return nil
			}
		}

		return NewAuthError(ErrAccessDenied,
			fmt.Sprintf("role required: one of %v", requiredRoles))
	}
}

// Helper functions

func stringSliceContains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func removeStringFromSlice(slice []string, str string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != str {
			result = append(result, s)
		}
	}
	return result
}

// matchPermission supports wildcard matching
// e.g., "tools:*" matches "tools:read", "tools:write", etc.
func matchPermission(pattern, permission string) bool {
	// Simple wildcard matching
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(permission) >= len(prefix) && permission[:len(prefix)] == prefix
	}
	return pattern == permission
}

// DefaultRBACProvider is a global RBAC provider for convenience
var defaultRBACProvider = NewRBACProvider(nil)

// AssignRole assigns a role to a user using the default provider
func AssignRole(userID, roleName string) error {
	return defaultRBACProvider.AssignRole(userID, roleName)
}

// HasPermission checks if a user has a permission using the default provider
func HasPermission(userID, permission string) bool {
	return defaultRBACProvider.HasPermission(userID, permission)
}

// CanAccessResource checks if a user can access a resource using the default provider
func CanAccessResource(userID, resource string) bool {
	return defaultRBACProvider.CanAccessResource(userID, resource)
}
