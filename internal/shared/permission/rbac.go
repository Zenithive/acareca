package permission

type RBACPolicy struct {
	rolePermissions map[Role]map[Resource]PermissionSet
}

func NewRBACPolicy() *RBACPolicy {
	return &RBACPolicy{
		rolePermissions: DefaultRolePermissions(),
	}
}

func (r *RBACPolicy) Check(ctx Context, resource Resource, action Action) CheckResult {
	resourcePerms, ok := r.rolePermissions[ctx.Role]
	if !ok {
		return Deny("role not found")
	}

	perms, ok := resourcePerms[resource]
	if !ok {
		return Deny("resource not accessible for this role")
	}

	if perms.HasPermission(action) {
		return Allow()
	}

	return Deny("action not permitted for this role")
}

func (r *RBACPolicy) GetPermissions(ctx Context, resource Resource) PermissionSet {
	resourcePerms, ok := r.rolePermissions[ctx.Role]
	if !ok {
		return PermissionSet{}
	}

	perms, ok := resourcePerms[resource]
	if !ok {
		return PermissionSet{}
	}

	return perms
}

func (r *RBACPolicy) SetRolePermissions(role Role, resource Resource, perms PermissionSet) {
	if r.rolePermissions[role] == nil {
		r.rolePermissions[role] = make(map[Resource]PermissionSet)
	}
	r.rolePermissions[role][resource] = perms
}

func DefaultRolePermissions() map[Role]map[Resource]PermissionSet {
	return map[Role]map[Resource]PermissionSet{
		RoleAccountant: {
			ResourceReport:        {Read: true, Create: false},
			ResourceUser:          {Read: true, Create: false},
			ResourceSalesPurchase: {Read: true, Create: false},
			ResourceLockDate:      {Read: true, Create: false},
		},
	}
}
