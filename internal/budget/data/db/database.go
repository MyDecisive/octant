package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql" // use mysql driver
	"github.com/mydecisive/octant/internal/config"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	greptimeDBSecretName        = "greptimedb-users-auth" // nolint:gosec // no, its not a cred
	greptimeDBSecretUserKey     = "GREPTIME_USER"         // nolint:gosec // no, its not a cred
	greptimeDBSecretPasswordKey = "GREPTIME_PASSWD"       // nolint:gosec // no, its not a cred
	greptimeDBSecretDBKey       = "GREPTIME_DATABASE"     // nolint:gosec // no, its not a cred
	greptimeDBSecretPortKey     = "GREPTIME_MYSQL_PORT"   // nolint:gosec // no, its not a cred
	greptimedbRootURLFormatter  = "%s.%s.svc.cluster.local"
)

var mustExists = []string{ //nolint:gochecknoglobals
	greptimeDBSecretUserKey,
	greptimeDBSecretPasswordKey,
	greptimeDBSecretDBKey,
	greptimeDBSecretPortKey,
}

var (
	ErrNotFound = errors.New("not found")
	ErrMissing  = errors.New("missing")
)

type Database struct {
	Namespace string
	DB        *sql.DB
}

func NewGreptimeDB(
	ctx context.Context,
	con *config.Configuration,
	k8sClient kubernetes.Interface,
	namespace string,
) (*Database, error) {
	secret, err := k8sClient.CoreV1().Secrets(namespace).Get(ctx, greptimeDBSecretName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, fmt.Errorf("%w:%w", ErrNotFound, err)
		}
		return nil, err
	}

	host := con.Budget.GreptimeDBURLOverride
	if host == "" {
		host = fmt.Sprintf(greptimedbRootURLFormatter, con.Budget.DefaultGreptimeDBName, namespace)
	}

	for _, key := range mustExists {
		if _, exists := secret.Data[key]; !exists {
			return nil, fmt.Errorf("%w:%s", ErrMissing, key)
		}
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		string(secret.Data[greptimeDBSecretUserKey]),
		string(secret.Data[greptimeDBSecretPasswordKey]),
		host,
		string(secret.Data[greptimeDBSecretPortKey]),
		string(secret.Data[greptimeDBSecretDBKey]),
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("connect to greptime:%w", err)
	}

	db.SetConnMaxLifetime(time.Duration(con.Budget.GreptimeDBMaxConnTimeout) * time.Minute)
	db.SetMaxOpenConns(con.Budget.GreptimeDBMaxConn)
	db.SetMaxIdleConns(con.Budget.GreptimeDBMaxConn)

	return &Database{
		Namespace: namespace,
		DB:        db,
	}, nil
}
