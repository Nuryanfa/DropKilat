package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"

	"github.com/Nuryanfa/DropKilat/migrations"
	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

func LatestVersion() (uint, error) {
	names, err := fs.Glob(migrations.Files, "*.up.sql")
	if err != nil {
		return 0, err
	}
	var latest uint
	for _, name := range names {
		number, _, _ := strings.Cut(name, "_")
		v, err := strconv.ParseUint(number, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid migration filename: %w", err)
		}
		if uint(v) > latest {
			latest = uint(v)
		}
	}
	if latest == 0 {
		return 0, errors.New("no embedded migrations")
	}
	return latest, nil
}

func Ready(ctx context.Context, p *pgxpool.Pool) error {
	if err := p.Ping(ctx); err != nil {
		return fmt.Errorf("database ping: %w", err)
	}
	latest, err := LatestVersion()
	if err != nil {
		return err
	}
	var version uint64
	var dirty bool
	if err := p.QueryRow(ctx, "SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty); err != nil {
		return fmt.Errorf("schema version: %w", err)
	}
	if dirty || version != uint64(latest) {
		return errors.New("schema is dirty or does not match binary")
	}
	return nil
}

func openMigrator(ctx context.Context, url string) (*migrate.Migrate, error) {
	config, err := pgx.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	config.ConnectTimeout = 5 * time.Second
	connection := stdlib.OpenDB(*config)
	if err := connection.PingContext(ctx); err != nil {
		return nil, errors.Join(err, connection.Close())
	}
	driver, err := migratepgx.WithInstance(connection, &migratepgx.Config{})
	if err != nil {
		return nil, errors.Join(err, connection.Close())
	}
	source, err := iofs.New(migrations.Files, ".")
	if err != nil {
		return nil, errors.Join(err, driver.Close())
	}
	m, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		return nil, errors.Join(err, source.Close(), driver.Close())
	}
	return m, nil
}

func closeMigrator(m *migrate.Migrate) error {
	sourceErr, databaseErr := m.Close()
	return errors.Join(sourceErr, databaseErr)
}

// Migrate uses the owner URL; down reverses one migration, never all implicitly.
func Migrate(ctx context.Context, url string, direction string) (err error) {
	if direction != "up" && direction != "down" && direction != "version" {
		return errors.New("migration direction must be up, down or version")
	}
	m, err := openMigrator(ctx, url)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, closeMigrator(m)) }()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			select {
			case m.GracefulStop <- true:
			case <-done:
			}
		case <-done:
		}
	}()
	switch direction {
	case "up":
		err = m.Up()
	case "down":
		err = m.Steps(-1)
	case "version":
		_, _, err = m.Version()
	}
	if errors.Is(err, migrate.ErrNoChange) {
		err = nil
	}
	return errors.Join(err, ctx.Err())
}

func Version(ctx context.Context, url string) (version uint, dirty bool, err error) {
	m, err := openMigrator(ctx, url)
	if err != nil {
		return 0, false, err
	}
	defer func() { err = errors.Join(err, closeMigrator(m)) }()
	return m.Version()
}
