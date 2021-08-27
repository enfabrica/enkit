package authz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/enfabrica/enkit/lib/authz/opa_files"
	"github.com/enfabrica/enkit/lib/authz/plugins"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
)

type regoInput struct {
	Username string   `json:"username"`
	Resource Resource `json:"resource"`
	Action   Action   `json:"action"`
}

func (i regoInput) JSON() string {
	js, _ := json.Marshal(i)
	return string(js)
}

type AuthService struct {
	q rego.PreparedEvalQuery
}

var (
	ErrorOnlyAccessSelfOwned = errors.New("permission denied to perform this action")
)

func (as *AuthService) Do(ctx context.Context, req *AuthRequest) error {
	i := &regoInput{
		Username: req.UserName,
		Resource: req.Resource,
		Action:   req.Action,
	}
	res, err := as.q.Eval(ctx, rego.EvalInput(i))
	if err != nil {
		return err
	}
	if len(res) == 0 {
		return ErrorOnlyAccessSelfOwned
	}

	if m, ok := res[0].Bindings["x"].(map[string]interface{}); ok {
		result := m["allow"]
		if canDo, ok := result.(bool); ok {
			if canDo {
				return nil
			} else {
				fmt.Println(i.JSON(), res)
				return ErrorOnlyAccessSelfOwned
			}
		}
	}
	return errors.New("undefined response given")
}

//this is a compile time assertion.

func NewService(d *plugins.DummyService) (*AuthService, error) {
	rego.RegisterBuiltin2(plugins.ResourceOwn(d))
	rego.RegisterBuiltin1(plugins.UserRole(d))
	rego.RegisterBuiltin2(plugins.ResourceRole(d))
	c := ast.MustCompileModules(opa_files.OpaFiles)
	q := rego.New(
		rego.Query(`x = data.rbac`),
		rego.Trace(true),
		rego.Compiler(c),
	)
	ctx := context.Background()
	preppedQuery, err := q.PrepareForEval(ctx)
	return &AuthService{
		q: preppedQuery,
	}, err
}
