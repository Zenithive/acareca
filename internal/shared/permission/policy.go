package permission

type Policy interface {
	Check(ctx Context, resource Resource, action Action) CheckResult
	GetPermissions(ctx Context, resource Resource) PermissionSet
}

type PolicyFunc func(ctx Context, resource Resource, action Action) CheckResult

func (f PolicyFunc) Check(ctx Context, resource Resource, action Action) CheckResult {
	return f(ctx, resource, action)
}

func (f PolicyFunc) GetPermissions(ctx Context, resource Resource) PermissionSet {
	return PermissionSet{
		Read:   f(ctx, resource, "read").Allowed,
		Create: f(ctx, resource, "create").Allowed,
	}
}

type CompositePolicy struct {
	policies []Policy
}

func NewCompositePolicy(policies ...Policy) *CompositePolicy {
	return &CompositePolicy{policies: policies}
}

func (cp *CompositePolicy) Check(ctx Context, resource Resource, action Action) CheckResult {
	for _, policy := range cp.policies {
		result := policy.Check(ctx, resource, action)
		if !result.Allowed {
			return result
		}
	}
	return Allow()
}

func (cp *CompositePolicy) GetPermissions(ctx Context, resource Resource) PermissionSet {
	if len(cp.policies) == 0 {
		return PermissionSet{}
	}

	perms := cp.policies[0].GetPermissions(ctx, resource)

	for i := 1; i < len(cp.policies); i++ {
		p := cp.policies[i].GetPermissions(ctx, resource)
		perms.Read = perms.Read && p.Read
		perms.Create = perms.Create && p.Create
	}

	return perms
}

type OrPolicy struct {
	policies []Policy
}

func NewOrPolicy(policies ...Policy) *OrPolicy {
	return &OrPolicy{policies: policies}
}

func (op *OrPolicy) Check(ctx Context, resource Resource, action Action) CheckResult {
	var lastDeny CheckResult
	for _, policy := range op.policies {
		result := policy.Check(ctx, resource, action)
		if result.Allowed {
			return result
		}
		lastDeny = result
	}
	return lastDeny
}

func (op *OrPolicy) GetPermissions(ctx Context, resource Resource) PermissionSet {
	perms := PermissionSet{}

	for _, policy := range op.policies {
		p := policy.GetPermissions(ctx, resource)
		perms.Read = perms.Read || p.Read
		perms.Create = perms.Create || p.Create
	}

	return perms
}
