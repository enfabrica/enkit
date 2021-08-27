package rbac

default allow = false

allow {
    user.root(input.username)
}
# Admins can performa
allow {
    admin_roles := ["admin"]
    some i
    resource.role(input.username, input.resource) == admin_roles[i]
}

allow {
    resource.own(input.username, input.resource)
}

allow {
    resource.own(input.username, input.resource)
}

allow {
    input.action == "read"
}




