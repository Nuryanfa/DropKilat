package db

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func integrationPool(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration suite requires Docker")
	}
	script, err := filepath.Abs("../../../deploy/init-db.sql")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	container, err := postgres.Run(ctx, "postgres:17-alpine", postgres.WithDatabase("dropkilat"), postgres.WithUsername("dropkilat"), postgres.WithPassword("integration-only"), postgres.WithInitScripts(script), postgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := container.Terminate(cleanup); err != nil {
			t.Error(err)
		}
	})
	ownerURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, ownerURL, "up"); err != nil {
		t.Fatal(err)
	}
	appURL, err := url.Parse(ownerURL)
	if err != nil {
		t.Fatal(err)
	}
	appURL.User = url.UserPassword("dropkilat_app", "dropkilat_app")
	pool, err := NewPool(ctx, appURL.String()+"&pool_max_conns=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool, ownerURL
}

func mustExec(t *testing.T, tx pgx.Tx, sql string, args ...any) pgconn.CommandTag {
	t.Helper()
	tag, err := tx.Exec(t.Context(), sql, args...)
	if err != nil {
		t.Fatal(err)
	}
	return tag
}
func countRows(t *testing.T, tx pgx.Tx, sql string, want int) {
	t.Helper()
	var got int
	if err := tx.QueryRow(t.Context(), sql).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s: got %d, want %d", sql, got, want)
	}
}
func checkCode(t *testing.T, err error, code string) {
	t.Helper()
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != code {
		t.Fatalf("expected SQLSTATE %s, got %v", code, err)
	}
}

func TestRLSIntegration(t *testing.T) {
	pool, ownerURL := integrationPool(t)
	ctx := t.Context()
	owner, err := NewPool(ctx, ownerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	// Exercise every down migration and then restore, using the same embedded source as the CLI.
	for i := 0; i < 2; i++ {
		if err := Migrate(ctx, ownerURL, "down"); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(ctx, ownerURL, "up"); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, ownerURL, "up"); err != nil {
		t.Fatal("up should be idempotent:", err)
	}
	version, dirty, err := Version(ctx, ownerURL)
	if err != nil || version != 2 || dirty {
		t.Fatalf("version %d dirty %t error %v", version, dirty, err)
	}
	if err := Ready(ctx, pool); err != nil {
		t.Fatal("app role cannot read clean schema_migrations:", err)
	}
	var superuser, bypass bool
	if err := pool.QueryRow(ctx, "SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(&superuser, &bypass); err != nil || superuser || bypass {
		t.Fatalf("unsafe app role: super=%t bypass=%t err=%v", superuser, bypass, err)
	}
	sellerA, sellerB, buyerA, buyerB := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	live, draft, variant, draftVariant := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	orderA, orderB := uuid.New(), uuid.New()
	err = WithSystemTx(ctx, pool, func(tx pgx.Tx) error {
		mustExec(t, tx, `INSERT INTO sellers (id, shop_name, slug, phone_enc, phone_hash) VALUES ($1,'Shop A','shop-a','a','a'),($2,'Shop B','shop-b','b','b')`, sellerA, sellerB)
		mustExec(t, tx, `INSERT INTO buyers (id,phone_enc,phone_hash,verified_at) VALUES ($1,'a','a','2026-11-12Z'),($2,'b','b','2026-11-12Z')`, buyerA, buyerB)
		mustExec(t, tx, `INSERT INTO drops (id,seller_id,slug,title,status,opens_at,closes_at) VALUES ($1,$2,'live-drop','Live drop','live','2026-11-12Z','2026-11-13Z'),($3,$4,'draft-drop','Draft drop','draft','2026-11-12Z','2026-11-13Z')`, live, sellerA, draft, sellerB)
		mustExec(t, tx, `INSERT INTO drop_variants (id,drop_id,seller_id,name,price,stock_total,available) VALUES ($1,$2,$3,'Live',20000,10,10),($4,$5,$6,'Draft',20000,10,10)`, variant, live, sellerA, draftVariant, draft, sellerB)
		mustExec(t, tx, `INSERT INTO orders (id,code,seller_id,drop_id,variant_id,buyer_id,qty,unit_price,amount,payment_mode,status,hold_expires_at) VALUES ($1,'ORDERAAA',$2,$3,$4,$5,1,20000,20000,'manual','held','2026-11-13Z'),($6,'ORDERBBB',$2,$3,$4,$7,1,20000,20000,'manual','held','2026-11-13Z')`, orderA, sellerA, live, variant, buyerA, orderB, buyerB)
		mustExec(t, tx, `INSERT INTO waitlist_entries (seller_id,drop_id,variant_id,buyer_id) VALUES ($1,$2,$3,$4),($1,$2,$3,$5)`, sellerA, live, variant, buyerA, buyerB)
		mustExec(t, tx, `INSERT INTO payments (order_id,seller_id,provider,amount,status) VALUES ($1,$2,'manual',20000,'pending'),($3,$2,'manual',20000,'pending')`, orderA, sellerA, orderB)
		mustExec(t, tx, `INSERT INTO drop_photos (drop_id,seller_id,path) VALUES ($1,$2,'live.jpg'),($3,$4,'draft.jpg')`, live, sellerA, draft, sellerB)
		mustExec(t, tx, `INSERT INTO seller_social_links (seller_id,platform,handle,verify_code,status) VALUES ($1,'instagram','shopa','code','verified'),($2,'instagram','shopb','code','pending')`, sellerA, sellerB)
		mustExec(t, tx, `INSERT INTO audit_log (actor_type,action,entity,entity_id) VALUES ('system','seed','test','fixture')`)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("public catalog and buyer spoof", func(t *testing.T) {
		err := WithPublicTx(ctx, pool, func(tx pgx.Tx) error {
			countRows(t, tx, "SELECT count(*) FROM drops", 1)
			countRows(t, tx, "SELECT count(*) FROM drop_variants", 1)
			countRows(t, tx, "SELECT count(*) FROM drop_photos", 1)
			countRows(t, tx, "SELECT count(*) FROM seller_social_links", 1)
			countRows(t, tx, "SELECT count(*) FROM orders", 0)
			countRows(t, tx, "SELECT count(*) FROM waitlist_entries", 0)
			countRows(t, tx, "SELECT count(*) FROM payments", 0)
			mustExec(t, tx, `SELECT set_config('app.buyer_id',$1,true)`, buyerA.String())
			countRows(t, tx, "SELECT count(*) FROM orders", 0)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("public transaction is read only", func(t *testing.T) {
		err := WithPublicTx(ctx, pool, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "UPDATE drop_variants SET name = 'forbidden'")
			return err
		})
		checkCode(t, err, "25006")
	})
	// Use the internal context setter with a writable transaction to isolate RLS policy behavior.
	// The production helper always begins read-only; no production API permits this.
	t.Run("public RLS independently rejects writes", func(t *testing.T) {
		err := withTx(ctx, pool, pgx.TxOptions{}, "public", "", "", func(tx pgx.Tx) error {
			if tag := mustExec(t, tx, "UPDATE drop_variants SET name = 'forbidden'"); tag.RowsAffected() != 0 {
				t.Fatal("public updated variants")
			}
			_, err := tx.Exec(ctx, `INSERT INTO orders (code,seller_id,drop_id,variant_id,buyer_id,qty,unit_price,amount,payment_mode,status) VALUES ('FORBID01',$1,$2,$3,$4,1,20000,20000,'manual','paid')`, sellerA, live, variant, buyerA)
			return err
		})
		checkCode(t, err, "42501")
	})
	t.Run("buyer own rows and writes", func(t *testing.T) {
		err := WithBuyerTx(ctx, pool, buyerA, func(tx pgx.Tx) error {
			countRows(t, tx, "SELECT count(*) FROM drops", 1)
			countRows(t, tx, "SELECT count(*) FROM orders", 1)
			countRows(t, tx, "SELECT count(*) FROM waitlist_entries", 1)
			countRows(t, tx, "SELECT count(*) FROM payments", 1)
			var got uuid.UUID
			if err := tx.QueryRow(ctx, "SELECT id FROM orders").Scan(&got); err != nil || got != orderA {
				t.Fatalf("wrong buyer order %s: %v", got, err)
			}
			if tag := mustExec(t, tx, "UPDATE orders SET recipient_name = 'Buyer A'"); tag.RowsAffected() != 1 {
				t.Fatal("own update did not affect one order")
			}
			if tag := mustExec(t, tx, "UPDATE drop_variants SET name = 'forbidden'"); tag.RowsAffected() != 0 {
				t.Fatal("buyer updated variant")
			}
			if tag := mustExec(t, tx, "UPDATE waitlist_entries SET status = 'left'"); tag.RowsAffected() != 1 {
				t.Fatal("buyer updated another waitlist")
			}
			mustExec(t, tx, `INSERT INTO reports (seller_id,order_id,buyer_id,kind,message) VALUES ($1,$2,$3,'other','Example report')`, sellerA, orderA, buyerA)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("buyer cannot transfer order", func(t *testing.T) {
		err := WithBuyerTx(ctx, pool, buyerA, func(tx pgx.Tx) error { _, err := tx.Exec(ctx, "UPDATE orders SET buyer_id = $1", buyerB); return err })
		checkCode(t, err, "42501")
	})
	t.Run("buyer cannot report another order", func(t *testing.T) {
		err := WithBuyerTx(ctx, pool, buyerA, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `INSERT INTO reports (seller_id,order_id,buyer_id,kind,message) VALUES ($1,$2,$3,'other','Forbidden report')`, sellerA, orderB, buyerA)
			return err
		})
		checkCode(t, err, "42501")
	})
	t.Run("seller isolation and pool cleanup", func(t *testing.T) {
		for _, scenario := range []struct {
			seller uuid.UUID
			orders int
		}{{sellerA, 2}, {sellerB, 0}} {
			err := WithSellerTx(ctx, pool, scenario.seller, func(tx pgx.Tx) error {
				countRows(t, tx, "SELECT count(*) FROM drops", 1)
				countRows(t, tx, "SELECT count(*) FROM orders", scenario.orders)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			var count int
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM drops").Scan(&count); err != nil || count != 0 {
				t.Fatalf("context leaked on one-connection pool: %d %v", count, err)
			}
		}
		err := WithSystemTx(ctx, pool, func(tx pgx.Tx) error {
			countRows(t, tx, "SELECT count(*) FROM orders", 2)
			countRows(t, tx, "SELECT count(*) FROM drops", 2)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM orders").Scan(&count); err != nil || count != 0 {
			t.Fatalf("system context leaked: %d %v", count, err)
		}
	})
	t.Run("audit append only", func(t *testing.T) {
		for _, sql := range []string{"UPDATE audit_log SET action = 'forbidden'", "DELETE FROM audit_log"} {
			err := WithSystemTx(ctx, pool, func(tx pgx.Tx) error { _, err := tx.Exec(ctx, sql); return err })
			checkCode(t, err, "42501")
		}
	})
	t.Run("readiness rejects wrong or dirty schema", func(t *testing.T) {
		for _, sql := range []string{"UPDATE schema_migrations SET dirty = true", "UPDATE schema_migrations SET version = 1", "UPDATE schema_migrations SET version = 3"} {
			if _, err := owner.Exec(ctx, sql); err != nil {
				t.Fatal(err)
			}
			if err := Ready(ctx, pool); err == nil {
				t.Fatal("unready schema accepted")
			}
			if _, err := owner.Exec(ctx, "UPDATE schema_migrations SET version = 2, dirty = false"); err != nil {
				t.Fatal(err)
			}
		}
	})
	t.Run("rollback and panic leave no state or identity", func(t *testing.T) {
		failure := errors.New("rollback probe")
		for _, panics := range []bool{false, true} {
			func() {
				if panics {
					defer func() {
						if recover() != failure {
							t.Error("panic was not preserved")
						}
					}()
				}
				err := WithSellerTx(ctx, pool, sellerA, func(tx pgx.Tx) error {
					mustExec(t, tx, "UPDATE orders SET recipient_name = 'must rollback'")
					if panics {
						panic(failure)
					}
					return failure
				})
				if panics || !errors.Is(err, failure) {
					t.Fatalf("wrong rollback error: %v", err)
				}
			}()
			var visible int
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM orders").Scan(&visible); err != nil || visible != 0 {
				t.Fatalf("identity leaked after rollback: %d %v", visible, err)
			}
			err := WithSystemTx(ctx, pool, func(tx pgx.Tx) error {
				countRows(t, tx, "SELECT count(*) FROM orders WHERE recipient_name = 'must rollback'", 0)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
	})
	t.Run("readiness rejects missing schema and unavailable pool", func(t *testing.T) {
		if _, err := owner.Exec(ctx, "ALTER TABLE schema_migrations RENAME TO schema_migrations_probe"); err != nil {
			t.Fatal(err)
		}
		missingErr := Ready(ctx, pool)
		if _, err := owner.Exec(ctx, "ALTER TABLE schema_migrations_probe RENAME TO schema_migrations"); err != nil {
			t.Fatal(err)
		}
		if missingErr == nil {
			t.Fatal("missing migration metadata accepted")
		}
		closed, err := NewPool(ctx, ownerURL)
		if err != nil {
			t.Fatal(err)
		}
		closed.Close()
		if err := Ready(ctx, closed); err == nil {
			t.Fatal("unavailable database accepted")
		}
	})
}
