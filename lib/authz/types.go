package authz

// In this example, a resource is either an ip address or it is a uuid
type Resource string

type Action string

// Basic Crud
var (
	ActionRead   = Action("read")
	ActionWrite  = Action("write")
	ActionEdit   = Action("edit")
	ActionCreate = Action("create")
	ActionSSH    = Action("ssh")
)



