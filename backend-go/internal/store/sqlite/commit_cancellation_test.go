package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	modernsqlite "modernc.org/sqlite"
)

// Gate the real SQLite commit after it has written the database, before its
// successful result reaches database/sql and Store.Write. No timing race or
// production test hook is needed to cancel precisely in that interval.
type commitGate struct {
	path               string
	driver             *modernsqlite.Driver
	enabled            atomic.Bool
	committed, release chan struct{}
}

func (g *commitGate) Driver() driver.Driver { return g.driver }
func (g *commitGate) Connect(context.Context) (driver.Conn, error) {
	conn, err := g.driver.Open(g.path)
	if err != nil {
		return nil, err
	}
	return &commitGateConn{Conn: conn, gate: g}, nil
}

type commitGateConn struct {
	driver.Conn
	gate *commitGate
}

func (c *commitGateConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *commitGateConn) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	return &commitGateTx{Tx: tx, gate: c.gate}, nil
}

type commitGateTx struct {
	driver.Tx
	gate *commitGate
}

func (tx *commitGateTx) Commit() error {
	err := tx.Tx.Commit()
	if err == nil && tx.gate.enabled.Load() {
		close(tx.gate.committed)
		<-tx.gate.release
	}
	return err
}

func TestWriteReportsSuccessWhenCallerCancelsAfterSQLiteCommit(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "commit-cancel.db")
	gate := &commitGate{path: path, driver: &modernsqlite.Driver{}, committed: make(chan struct{}), release: make(chan struct{})}
	writer := sql.OpenDB(gate)
	writer.SetMaxOpenConns(1)
	_, err := writer.ExecContext(ctx, "create table example(id text primary key)")
	require.NoError(t, err)
	reader, err := OpenReadOnly(ctx, path, time.Second, 1)
	require.NoError(t, err)
	storeCtx, cancelStore := context.WithCancel(ctx)
	store := &Store{writer: writer, reader: reader, ctx: storeCtx, cancel: cancelStore, writes: make(chan *writeRequest, 1), done: make(chan struct{})}
	go store.writeLoop()
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	gate.enabled.Store(true)
	writeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- store.Write(writeCtx, func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "insert into example(id) values('committed')")
			return err
		})
	}()
	<-gate.committed
	cancel()
	close(gate.release)
	require.NoError(t, <-result, "a committed transaction must never be reported as canceled")
	var count int
	require.NoError(t, reader.QueryRowContext(ctx, "select count(*) from example").Scan(&count))
	require.Equal(t, 1, count)
}
