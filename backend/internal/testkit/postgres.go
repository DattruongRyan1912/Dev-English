// Package testkit contains opt-in helpers for PostgreSQL integration tests.
package testkit

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// DatabaseURLEnv is deliberately separate from DATABASE_URL so tests do
	// not accidentally connect to an application or production database.
	DatabaseURLEnv     = "DEVENGLISH_TEST_DATABASE_URL"
	defaultPingTimeout = 30 * time.Second
)

var (
	ErrDatabaseURLNotConfigured = errors.New("test database URL is not configured")
	ErrInvalidDatabaseURL       = errors.New("test database URL is invalid")
)

// Config describes an already-provisioned PostgreSQL test target. The testkit
// never creates databases, containers, schemas, or migrations.
type Config struct {
	DatabaseURL string
	PingTimeout time.Duration
}

// ConfigFromEnv reads only the opt-in integration-test URL. An empty URL is
// expected for the normal unit-test run.
func ConfigFromEnv() Config {
	return Config{
		DatabaseURL: strings.TrimSpace(os.Getenv(DatabaseURLEnv)),
		PingTimeout: defaultPingTimeout,
	}
}

// Enabled reports whether an integration target was explicitly configured.
func (c Config) Enabled() bool {
	return strings.TrimSpace(c.DatabaseURL) != ""
}

func (c Config) normalized() Config {
	if c.PingTimeout <= 0 {
		c.PingTimeout = defaultPingTimeout
	}
	c.DatabaseURL = strings.TrimSpace(c.DatabaseURL)
	return c
}

func (c Config) validate() error {
	if c.DatabaseURL == "" {
		return ErrDatabaseURLNotConfigured
	}
	parsed, err := url.Parse(c.DatabaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ErrInvalidDatabaseURL
	}
	return nil
}

// Fixture owns a pool connected to an already-running PostgreSQL test
// database. Callers remain responsible for applying migrations and cleaning
// up rows created by their test.
type Fixture struct {
	Pool   *pgxpool.Pool
	Config Config
}

// OpenPostgres connects to and pings the configured existing database. It
// does not provision a database, start a container, or apply migrations.
func OpenPostgres(ctx context.Context, config Config) (*Fixture, error) {
	config = config.normalized()
	if err := config.validate(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	pool, err := pgxpool.New(ctx, config.DatabaseURL)
	if err != nil {
		return nil, ErrInvalidDatabaseURL
	}

	pingContext, cancel := context.WithTimeout(ctx, config.PingTimeout)
	defer cancel()
	if err := pool.Ping(pingContext); err != nil {
		pool.Close()
		// Do not return the provider/database URL or driver details to test
		// output; callers only need to know that the target is unreachable.
		return nil, errors.New("test database is unreachable")
	}
	return &Fixture{Pool: pool, Config: config}, nil
}

// Close releases the fixture's pool. It is safe to call on a nil fixture.
func (f *Fixture) Close() {
	if f != nil && f.Pool != nil {
		f.Pool.Close()
	}
}

// RequirePostgres skips when the opt-in URL is absent and registers cleanup
// when a caller has supplied an already-provisioned integration database.
func RequirePostgres(t testing.TB) *Fixture {
	t.Helper()
	fixture, err := OpenPostgres(context.Background(), ConfigFromEnv())
	if errors.Is(err, ErrDatabaseURLNotConfigured) {
		t.Skip("set DEVENGLISH_TEST_DATABASE_URL to run PostgreSQL integration coverage")
	}
	if err != nil {
		t.Fatalf("open PostgreSQL test database: %v", err)
	}
	t.Cleanup(fixture.Close)
	return fixture
}
