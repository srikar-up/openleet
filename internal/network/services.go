package network

import (
	"context"
	"fmt"
)

// Services aggregates OpenLeet network clients for authentication and remote storage.
type Services struct {
	Config    *Config
	Auth      *AuthService
	Firestore *FirestoreService
}

// NewServices initializes network services from Config and Options.
// It constructs both AuthService and FirestoreService using their respective configuration parameters.
func NewServices(ctx context.Context, cfg *Config, opts ...Option) (*Services, error) {
	if cfg == nil {
		return nil, fmt.Errorf("%w: config cannot be nil", ErrInvalidInput)
	}

	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}

	auth, err := NewAuthService(cfg.FirebaseAPIKey, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize auth service: %w", err)
	}

	firestore, err := NewFirestoreService(cfg.FirebaseProjectID, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize firestore service: %w", err)
	}

	return &Services{
		Config:    cfg,
		Auth:      auth,
		Firestore: firestore,
	}, nil
}
