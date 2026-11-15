package domain

const (
	PermissionRead     = "read"
	PermissionDownload = "download"
	PermissionCreate   = "create"
	PermissionEdit     = "edit"
	PermissionDelete   = "delete"
)

func AllPermissions() []string {
	return []string{PermissionRead, PermissionDownload, PermissionCreate, PermissionEdit, PermissionDelete}
}

func (u User) IsAdmin() bool { return u.Role == RoleAdmin }

func (u User) Can(permission string) bool {
	if !u.Enabled {
		return false
	}
	valid := false
	for _, p := range AllPermissions() {
		if p == permission {
			valid = true
			break
		}
	}
	if !valid {
		return false
	}
	return u.Role == RoleAdmin || u.Role == RoleEditor
}

type ManagedUser = User

type CreateUserInput struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
	Role        string `json:"role"`
}

type UpdateUserInput struct {
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	Enabled     bool   `json:"enabled"`
}

type ResetPasswordInput struct {
	Password string `json:"password"`
}

type AuditEntry struct {
	OwnerUserID   *int64 `db:"owner_user_id" json:"ownerUserId"`
	OwnerName     string `db:"owner_name" json:"ownerName"`
	ID            int64  `db:"id" json:"id"`
	Action        string `db:"action" json:"action"`
	EntityType    string `db:"entity_type" json:"entityType"`
	EntityID      int64  `db:"entity_id" json:"entityId"`
	EntityName    string `db:"entity_name" json:"entityName"`
	Detail        string `db:"detail" json:"detail"`
	Actor         string `db:"actor" json:"actor"`
	ActorID       *int64 `db:"actor_id" json:"actorId"`
	ActorRoleID   *int64 `db:"actor_role_id" json:"actorRoleId"`
	ActorRoleName string `db:"actor_role_name" json:"actorRoleName"`
	RoleID        *int64 `db:"role_id" json:"roleId"`
	RoleName      string `db:"role_name" json:"roleName"`
	Result        string `db:"result" json:"result"`
	CreatedAt     string `db:"created_at" json:"createdAt"`
}

type AuditFilter struct {
	UserID     *int64
	Action     string
	Result     string
	EntityType string
	From       string
	To         string
	Page       int
	Size       int
}
