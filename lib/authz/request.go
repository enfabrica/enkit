package authz

import "context"

func (as *AuthService) NewRequest() *AuthRequest {
	return &AuthRequest{AuthService: as}
}

type AuthRequest struct {
	UserName  string
	Resource Resource
	Action    Action
	AuthService  *AuthService
}

func (a *AuthRequest) WithAction(act Action) *AuthRequest {
	a.Action = act
	return a
}

func (a *AuthRequest) OnResource(resource string) *AuthRequest {
	a.Resource = Resource(resource)
	return a
}

func (a *AuthRequest) Verify(ctx context.Context) error {
	return a.AuthService.Do(ctx, a)
}

func (a *AuthRequest) AsUser(username string) *AuthRequest {
	a.UserName = username
	return a
}
