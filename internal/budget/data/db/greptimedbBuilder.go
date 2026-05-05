package db

import (
	"context"

	"github.com/mydecisive/octant/internal/config"
	"k8s.io/client-go/kubernetes"
)

type DatabaseAccessBuilder interface {
	// Build returns a database access for the given namespace.
	Build(ctx context.Context, namespace string) (*Database, error)
}

type GreptimeDBBuilder struct {
	config    *config.Configuration
	k8sClient kubernetes.Interface

	existent map[string]*Database
}

// Ensure GreptimeDBBuilder implements DatabaseAccessBuilder.
var _ DatabaseAccessBuilder = &GreptimeDBBuilder{}

// NewGreptimeDBBuilder creates a new instance of GreptimeDBBuilder.
func NewGreptimeDBBuilder(con *config.Configuration, k8sClient kubernetes.Interface) *GreptimeDBBuilder {
	return &GreptimeDBBuilder{
		config:    con,
		k8sClient: k8sClient,
		existent:  map[string]*Database{},
	}
}

// Build returns a database access for the given namespace.
// If an access was created before, the existent one will be returned.
// Otherwise, a new one will be created.
func (dbb *GreptimeDBBuilder) Build(ctx context.Context, namespace string) (*Database, error) {
	if val, ok := dbb.existent[namespace]; ok {
		return val, nil
	}

	db, err := NewGreptimeDB(ctx, dbb.config, dbb.k8sClient, namespace)
	if err != nil {
		return nil, err
	}

	dbb.existent[namespace] = db
	return db, nil
}
