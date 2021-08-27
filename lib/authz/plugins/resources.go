package plugins

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/types"
)

type Role string

var (
	RoleUser  = Role("user")
	RoleAdmin = Role("admin")
)

type DummyUser struct {
	Name string
	Owns string
	Root bool
}

type DummyService struct {
	Users []DummyUser
}

func (s DummyService) IsRoot(username string) (bool, error) {
	return s.findUser(username).Root, nil
}

func (s DummyService) DoesOwnResource(username, resource string) (bool, error) {
	return s.findUser(username).Owns == resource, nil
}

func (s DummyService) RoleOnResource(username, resource string) (string, error) {
	if s.findUser(username).Owns == resource {
		return string(RoleAdmin), nil
	}
	return string(RoleUser), nil
}

func (s DummyService) findUser(username string) DummyUser {
	for _, v := range s.Users {
		if v.Name == username {
			return v
		}
	}
	return DummyUser{}
}

func ResourceOwn(d *DummyService) (*rego.Function, rego.Builtin2) {
	return &rego.Function{
			Name:    "resource.own",
			Memoize: true,
			Decl:    types.NewFunction(types.Args(types.S, types.S), types.B),
		}, func(bctx rego.BuiltinContext, op1 *ast.Term, op2 *ast.Term) (*ast.Term, error) {
			var userName string
			if err := ast.As(op1.Value, &userName); err != nil {
				return nil, err
			}
			var resource string
			if err := ast.As(op2.Value, &resource); err != nil {
				return nil, err
			}
			b, err := d.DoesOwnResource(userName, resource)
			return ast.BooleanTerm(b), err
		}
}

func ResourceRole(d *DummyService) (*rego.Function, rego.Builtin2) {
	return &rego.Function{
			Name:    "resource.role",
			Memoize: true,
			Decl:    types.NewFunction(types.Args(types.S, types.S), types.S),
		}, func(bctx rego.BuiltinContext, op1 *ast.Term, op2 *ast.Term) (*ast.Term, error) {
			var userName string
			if err := ast.As(op1.Value, &userName); err != nil {
				return nil, err
			}
			var resource string
			if err := ast.As(op2.Value, &resource); err != nil {
				return nil, err
			}
			b, err := d.RoleOnResource(userName, resource)
			return ast.StringTerm(b), err
		}
}

func UserRole(d *DummyService) (*rego.Function, rego.Builtin1) {
	return &rego.Function{
			Name:    "user.root",
			Memoize: true,
			Decl:    types.NewFunction(types.Args(types.S), types.B),
		}, func(bctx rego.BuiltinContext, op1 *ast.Term) (*ast.Term, error) {
			var userName string
			if err := ast.As(op1.Value, &userName); err != nil {
				return nil, err
			}
			b, err := d.IsRoot(userName)
			return ast.BooleanTerm(b), err
		}
}
