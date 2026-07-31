package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrClosed    = errors.New("sqlite store is closed")
	ErrQueueFull = errors.New("sqlite write queue is full")
	ErrNilWrite  = errors.New("sqlite write function is nil")
)

type StoreConfig struct {
	Path               string
	BusyTimeout        time.Duration
	ReadConnections    int
	WriteQueueCapacity int
}

type Store struct {
	writer *sql.DB
	reader *sql.DB

	ctx    context.Context
	cancel context.CancelFunc
	writes chan writeRequest
	done   chan struct{}

	stateMu sync.RWMutex
	closed  bool
}

type writeRequest struct {
	ctx    context.Context
	write  func(context.Context, *sql.Tx) error
	result chan error
}

func OpenStore(ctx context.Context, config StoreConfig) (*Store, error) {
	if config.ReadConnections < 1 {
		config.ReadConnections = 2
	}
	if config.WriteQueueCapacity < 1 {
		config.WriteQueueCapacity = 100
	}
	if config.BusyTimeout <= 0 {
		config.BusyTimeout = 5 * time.Second
	}

	writer, err := Open(ctx, config.Path, config.BusyTimeout, 1)
	if err != nil {
		return nil, fmt.Errorf("open SQLite writer: %w", err)
	}
	reader, err := OpenReadOnly(ctx, config.Path, config.BusyTimeout, config.ReadConnections)
	if err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("open SQLite reader: %w", err)
	}

	storeCtx, cancel := context.WithCancel(context.Background())
	store := &Store{
		writer: writer,
		reader: reader,
		ctx:    storeCtx,
		cancel: cancel,
		writes: make(chan writeRequest, config.WriteQueueCapacity),
		done:   make(chan struct{}),
	}
	go store.writeLoop()
	return store, nil
}

func (s *Store) Reader() *sql.DB {
	return s.reader
}

// Write runs write inside the store's single writer goroutine. A nil return
// means the transaction has committed; failed and cancelled writes are rolled
// back before the result is delivered.
func (s *Store) Write(ctx context.Context, write func(context.Context, *sql.Tx) error) error {
	if write == nil {
		return ErrNilWrite
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	req := writeRequest{ctx: ctx, write: write, result: make(chan error, 1)}

	s.stateMu.RLock()
	if s.closed {
		s.stateMu.RUnlock()
		return ErrClosed
	}
	select {
	case s.writes <- req:
		s.stateMu.RUnlock()
	default:
		s.stateMu.RUnlock()
		return ErrQueueFull
	}

	select {
	case err := <-req.result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		select {
		case err := <-req.result:
			return err
		default:
			return ErrClosed
		}
	}
}

func (s *Store) Close() error {
	s.stateMu.Lock()
	if s.closed {
		s.stateMu.Unlock()
		return nil
	}
	s.closed = true
	s.cancel()
	s.stateMu.Unlock()

	<-s.done
	readerErr := s.reader.Close()
	writerErr := s.writer.Close()
	return errors.Join(readerErr, writerErr)
}

func (s *Store) writeLoop() {
	defer close(s.done)
	for {
		if s.ctx.Err() != nil {
			s.rejectPendingWrites()
			return
		}
		select {
		case <-s.ctx.Done():
			s.rejectPendingWrites()
			return
		case req := <-s.writes:
			if s.ctx.Err() != nil {
				req.result <- ErrClosed
				s.rejectPendingWrites()
				return
			}
			req.result <- s.executeWrite(req)
		}
	}
}

func (s *Store) executeWrite(req writeRequest) (err error) {
	ctx, cancel := context.WithCancel(req.ctx)
	stopStoreCancel := context.AfterFunc(s.ctx, cancel)
	defer func() {
		stopStoreCancel()
		cancel()
	}()

	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		if s.ctx.Err() != nil {
			return ErrClosed
		}
		return fmt.Errorf("begin SQLite write transaction: %w", err)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback()
			err = fmt.Errorf("panic in SQLite write transaction: %v", recovered)
		}
	}()

	if err := req.write(ctx, tx); err != nil {
		_ = tx.Rollback()
		if s.ctx.Err() != nil {
			return ErrClosed
		}
		return err
	}
	if err := ctx.Err(); err != nil {
		_ = tx.Rollback()
		if s.ctx.Err() != nil {
			return ErrClosed
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		if s.ctx.Err() != nil {
			return ErrClosed
		}
		return fmt.Errorf("commit SQLite write transaction: %w", err)
	}
	return nil
}

func (s *Store) rejectPendingWrites() {
	for {
		select {
		case req := <-s.writes:
			req.result <- ErrClosed
		default:
			return
		}
	}
}
