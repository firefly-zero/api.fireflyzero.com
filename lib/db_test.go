package lib_test

import (
	"context"
	"sync"

	"github.com/firefly-zero/api.fireflyzero.com/lib/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// A wrapper around DB connection that is safe to be used concurrently.
//
// It is similar to pgxpool except it keeps only a single connection,
// and so it can be safely rolled back. It is slower than pgxpool
// so it must be used only in tests.
type BlockingDB struct {
	tx db.DBTX
	mx sync.Mutex
}

func NewBlockingDB(tx db.DBTX) *BlockingDB {
	return &BlockingDB{tx: tx}
}

func (db *BlockingDB) Map(f func(db.DBTX)) {
	db.mx.Lock()
	defer db.mx.Unlock()
	f(db.tx)
}

func (db *BlockingDB) Exec(ctx context.Context, q string, args ...any) (pgconn.CommandTag, error) {
	db.mx.Lock()
	defer db.mx.Unlock()
	return db.tx.Exec(ctx, q, args...)
}

func (db *BlockingDB) Query(ctx context.Context, q string, args ...any) (pgx.Rows, error) {
	db.mx.Lock()
	rows, err := db.tx.Query(ctx, q, args...)
	if err != nil {
		db.mx.Unlock()
		return nil, err
	}
	return &Rows{Rows: rows, mx: &db.mx}, nil
}

func (db *BlockingDB) QueryRow(ctx context.Context, q string, args ...any) pgx.Row {
	db.mx.Lock()
	row := db.tx.QueryRow(ctx, q, args...)
	return &Row{Row: row, mx: &db.mx}
}

type Rows struct {
	pgx.Rows
	once sync.Once
	mx   *sync.Mutex
}

func (r *Rows) Close() {
	r.Rows.Close()
	r.release()
}

func (r *Rows) Next() bool {
	more := r.Rows.Next()
	if !more {
		r.release()
	}
	return more
}

func (r *Rows) release() {
	r.once.Do(r.mx.Unlock)
}

type Row struct {
	pgx.Row
	once sync.Once
	mx   *sync.Mutex
}

func (r *Row) Scan(dest ...any) error {
	defer r.release()
	return r.Row.Scan(dest...)
}

func (r *Row) release() {
	r.once.Do(r.mx.Unlock)
}
