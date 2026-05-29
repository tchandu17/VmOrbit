package main

import (
	"log"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/vmOrbit/backend/internal/config"
	"github.com/vmOrbit/backend/internal/domain/model"
)

// permDef defines a permission to seed.
type permDef struct {
	Resource string
	Action   string
}

// roleDef defines a role and its permissions.
type roleDef struct {
	Name        string
	Description string
	Perms       []string // "resource:action"
}

// allPermissions is the canonical list of permissions in the system.
var allPermissions = []permDef{
	// VM operations
	{Resource: "vm", Action: "read"},
	{Resource: "vm", Action: "write"},
	{Resource: "vm", Action: "delete"},
	{Resource: "vm", Action: "power"},
	{Resource: "vm", Action: "snapshot"},
	{Resource: "vm", Action: "bulk"},
	{Resource: "vm", Action: "console"},
	// Hypervisor / provider management
	{Resource: "hypervisor", Action: "read"},
	{Resource: "hypervisor", Action: "write"},
	{Resource: "hypervisor", Action: "delete"},
	// User management
	{Resource: "user", Action: "read"},
	{Resource: "user", Action: "write"},
	{Resource: "user", Action: "delete"},
	// Role management
	{Resource: "role", Action: "read"},
	{Resource: "role", Action: "write"},
	{Resource: "role", Action: "delete"},
	// Tasks
	{Resource: "task", Action: "read"},
	{Resource: "task", Action: "write"},
	// Audit logs
	{Resource: "audit", Action: "read"},
	// Tags
	{Resource: "tag", Action: "read"},
	{Resource: "tag", Action: "write"},
	{Resource: "tag", Action: "delete"},
	// Policy governance
	{Resource: "policy", Action: "read"},
	{Resource: "policy", Action: "write"},
	{Resource: "policy", Action: "delete"},
	// Approval workflows
	{Resource: "approval", Action: "read"},
	{Resource: "approval", Action: "write"},
}

// defaultRoles defines the four built-in roles.
var defaultRoles = []roleDef{
	{
		Name:        "super_admin",
		Description: "Full unrestricted access to all resources",
		Perms: []string{
			"vm:read", "vm:write", "vm:delete", "vm:power", "vm:snapshot", "vm:bulk", "vm:console",
			"hypervisor:read", "hypervisor:write", "hypervisor:delete",
			"user:read", "user:write", "user:delete",
			"role:read", "role:write", "role:delete",
			"task:read", "task:write",
			"audit:read",
			"tag:read", "tag:write", "tag:delete",
			"policy:read", "policy:write", "policy:delete",
			"approval:read", "approval:write",
		},
	},
	{
		Name:        "admin",
		Description: "Manage hypervisors, VMs, users, and tags. Cannot manage roles.",
		Perms: []string{
			"vm:read", "vm:write", "vm:delete", "vm:power", "vm:snapshot", "vm:bulk", "vm:console",
			"hypervisor:read", "hypervisor:write", "hypervisor:delete",
			"user:read", "user:write",
			"role:read",
			"task:read", "task:write",
			"audit:read",
			"tag:read", "tag:write", "tag:delete",
			"policy:read", "policy:write", "policy:delete",
			"approval:read", "approval:write",
		},
	},
	{
		Name:        "operator",
		Description: "Operate VMs (power, snapshots, console). Read-only on providers and users.",
		Perms: []string{
			"vm:read", "vm:write", "vm:power", "vm:snapshot", "vm:console",
			"hypervisor:read",
			"user:read",
			"role:read",
			"task:read",
			"audit:read",
			"tag:read", "tag:write",
			"policy:read",
			"approval:read", "approval:write",
		},
	},
	{
		Name:        "readonly",
		Description: "Read-only access to VMs, hypervisors, tasks, and audit logs.",
		Perms: []string{
			"vm:read",
			"hypervisor:read",
			"user:read",
			"role:read",
			"task:read",
			"audit:read",
			"tag:read",
			"policy:read",
			"approval:read",
		},
	},
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := gorm.Open(postgres.Open(cfg.Database.DSN()), &gorm.Config{})
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	// ── Seed permissions ──────────────────────────────────────────────────────
	permMap := make(map[string]*model.Permission) // "resource:action" → Permission
	for _, pd := range allPermissions {
		var perm model.Permission
		key := pd.Resource + ":" + pd.Action
		result := db.Where("resource = ? AND action = ?", pd.Resource, pd.Action).First(&perm)
		if result.Error != nil {
			perm = model.Permission{Resource: pd.Resource, Action: pd.Action}
			if err := db.Create(&perm).Error; err != nil {
				log.Fatalf("create permission %s: %v", key, err)
			}
			log.Printf("  ✓ permission created: %s", key)
		} else {
			log.Printf("  · permission exists:  %s", key)
		}
		permMap[key] = &perm
	}

	// ── Seed roles ────────────────────────────────────────────────────────────
	roleMap := make(map[string]*model.Role)
	for _, rd := range defaultRoles {
		var role model.Role
		result := db.Preload("Permissions").Where("name = ?", rd.Name).First(&role)
		if result.Error != nil {
			role = model.Role{Name: rd.Name, Description: rd.Description}
			if err := db.Create(&role).Error; err != nil {
				log.Fatalf("create role %s: %v", rd.Name, err)
			}
			log.Printf("  ✓ role created: %s", rd.Name)
		} else {
			log.Printf("  · role exists:  %s", rd.Name)
		}

		// Sync permissions
		var desired []model.Permission
		for _, pk := range rd.Perms {
			if p, ok := permMap[pk]; ok {
				desired = append(desired, *p)
			}
		}
		if err := db.Model(&role).Association("Permissions").Replace(desired); err != nil {
			log.Fatalf("sync permissions for role %s: %v", rd.Name, err)
		}
		roleMap[rd.Name] = &role
	}

	// ── Seed admin user ───────────────────────────────────────────────────────
	email := "admin@example.com"
	username := "admin"
	password := "Admin1234!"

	var existing model.User
	if err := db.Preload("Roles").Where("email = ?", email).First(&existing).Error; err == nil {
		log.Printf("User %s already exists (id: %s)", email, existing.ID)
		// Ensure super_admin role is assigned
		hasSuperAdmin := false
		for _, r := range existing.Roles {
			if r.Name == "super_admin" {
				hasSuperAdmin = true
				break
			}
		}
		if !hasSuperAdmin {
			if superAdmin, ok := roleMap["super_admin"]; ok {
				if err := db.Model(&existing).Association("Roles").Append(superAdmin); err != nil {
					log.Fatalf("assign super_admin to existing user: %v", err)
				}
				log.Printf("  ✓ assigned super_admin role to existing admin user")
			}
		}
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("bcrypt: %v", err)
	}

	user := model.User{
		Email:        email,
		Username:     username,
		PasswordHash: string(hash),
		FirstName:    "Admin",
		LastName:     "User",
		IsActive:     true,
		IsVerified:   true,
	}

	if err := db.Create(&user).Error; err != nil {
		log.Fatalf("create user: %v", err)
	}

	// Assign super_admin role
	if superAdmin, ok := roleMap["super_admin"]; ok {
		if err := db.Model(&user).Association("Roles").Append(superAdmin); err != nil {
			log.Fatalf("assign super_admin role: %v", err)
		}
	}

	log.Printf("✓ Admin user created successfully!")
	log.Printf("  Email:    %s", email)
	log.Printf("  Password: %s", password)
	log.Printf("  ID:       %s", user.ID)
	log.Printf("  Role:     super_admin")
}
