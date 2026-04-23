package users

import "context"

// Repository describes the users persistence interface consumed by the auth
// module. Only the methods required by the auth flow are exposed here; the
// full users module will extend this interface when built.
type Repository interface {
	CreateUser(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByPhone(ctx context.Context, phone string) (*User, error)
	GetBySocialID(ctx context.Context, provider, socialID string) (*User, error)
}
