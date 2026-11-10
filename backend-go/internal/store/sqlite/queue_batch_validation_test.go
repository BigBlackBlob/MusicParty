package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	modernsqlite "modernc.org/sqlite"
)

type queueQueryCounter struct {
	path   string
	driver *modernsqlite.Driver
	count  atomic.Int32
}

func (c *queueQueryCounter) Driver() driver.Driver { return c.driver }
func (c *queueQueryCounter) Connect(context.Context) (driver.Conn, error) {
	conn, err := c.driver.Open(c.path)
	if err != nil {
		return nil, err
	}
	return &queueCountingConn{Conn: conn, counter: c}, nil
}

type queueCountingConn struct {
	driver.Conn
	counter *queueQueryCounter
}

func (c *queueCountingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if strings.Contains(query, "from local_track where status='DELETED' and id in") {
		c.counter.count.Add(1)
	}
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
}

func TestQueueTombstoneChecksAreBatchedAndDeduplicated(t *testing.T) {
	ctx := context.Background()
	counter := &queueQueryCounter{path: filepath.Join(t.TempDir(), "queries.db"), driver: &modernsqlite.Driver{}}
	db := sql.OpenDB(counter)
	defer db.Close()
	_, err := db.ExecContext(ctx, "create table local_track(id text primary key,status text)")
	require.NoError(t, err)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	items := make([]QueueItem, 0, 2000)
	for i := range 1000 {
		item := QueueItem{Music: Music{ID: fmt.Sprint(i), Platform: "local"}}
		items = append(items, item, item)
	}
	require.NoError(t, requireQueueTracksNotDeleted(ctx, tx, items))
	require.EqualValues(t, 2, counter.count.Load(), "1000 distinct local IDs need two bounded queries, even with duplicates")
	_, err = tx.ExecContext(ctx, "insert into local_track values('750','DELETED')")
	require.NoError(t, err)
	require.ErrorIs(t, requireQueueTracksNotDeleted(ctx, tx, items), ErrLocalTrackGone, "a tombstone in the second batch must still reject the write")
}
