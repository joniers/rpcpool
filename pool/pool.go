package pool

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

type baseErr struct {
	msg string
}

func (err *baseErr) Error() string {
	return err.msg
}

var Logger = log.New(ioutil.Discard, "rpc: ", log.LstdFlags)

// ObjectPool is a generic object pool
type ObjectPool struct {
	closed       bool
	closeLock    sync.Mutex
	evictionLock sync.Mutex
	factory      PooledObjectFactory
}

func NewPooledObject(object interface{}) *PooledObject {
	return &PooledObject{Object: object, UsedAt: time.Now()}
}

var timers = sync.Pool{
	New: func() interface{} {
		return time.NewTimer(0)
	},
}

// PoolStats contains pool state information and accumulated stats.
// TODO: remove Waits
type PoolStats struct {
	Requests uint32 // number of times a connection was requested by the pool
	Hits     uint32 // number of times free connection was found in the pool
	Waits    uint32 // number of times the pool had to wait for a connection
	Timeouts uint32 // number of times a wait timeout occurred

	TotalConns uint32 // the number of total connections in the pool
	FreeConns  uint32 // the number of free connections in the pool
}
type PooledObject struct {
	Object interface{}
	UsedAt time.Time
}

var (
	ErrClosed      = errors.New("rpc: client is closed")
	ErrPoolTimeout = errors.New("rpc: connection pool timeout")

	errConnClosed = errors.New("connection is closed")
	errConnStale  = errors.New("connection is stale")
)

type Pooler interface {
	Get() (*PooledObject, error)
	Return(*PooledObject) error
	Remove(*PooledObject, error) error
	Len() int
	FreeLen() int
	Stats() *PoolStats
	Close() error
	Closed() bool
}
type ConnPool struct {
	factory PooledObjectFactory
	OnClose func(*PooledObject) error

	poolTimeout time.Duration
	idleTimeout time.Duration

	queue chan struct{}

	connsMu sync.Mutex
	conns   []*PooledObject

	freeConnsMu sync.Mutex
	freeConns   []*PooledObject

	stats PoolStats

	_closed int32 // atomic
	lastErr atomic.Value
}

func NewConnPool(poolSize int, poolTimeout time.Duration, idleTimeout time.Duration, idleCheckFrequency time.Duration, factory PooledObjectFactory) *ConnPool {
	p := &ConnPool{
		factory:     factory,
		poolTimeout: poolTimeout,
		idleTimeout: idleTimeout,

		queue:     make(chan struct{}, poolSize),
		conns:     make([]*PooledObject, 0, poolSize),
		freeConns: make([]*PooledObject, 0, poolSize),
	}
	for i := 0; i < poolSize; i++ {
		p.queue <- struct{}{}
	}
	if idleTimeout > 0 && idleCheckFrequency > 0 {
		go p.reaper(idleCheckFrequency)
	}
	return p
}
func (p *ConnPool) Closed() bool {
	return atomic.LoadInt32(&p._closed) == 1
}
func (p *ConnPool) popFree() *PooledObject {
	if len(p.freeConns) == 0 {
		return nil
	}

	idx := len(p.freeConns) - 1
	cn := p.freeConns[idx]
	p.freeConns = p.freeConns[:idx]
	return cn
}

func (cn *PooledObject) IsStale(timeout time.Duration) bool {
	return timeout > 0 && time.Since(cn.UsedAt) > timeout
}

// Get returns existed connection from the pool or creates a new one.
func (p *ConnPool) Get() (*PooledObject, error) {
	if p.Closed() {
		return nil, ErrClosed
	}

	atomic.AddUint32(&p.stats.Requests, 1)

	timer := timers.Get().(*time.Timer)
	if !timer.Reset(p.poolTimeout) {
		<-timer.C
	}

	select {
	case <-p.queue:
		timers.Put(timer)
	case <-timer.C:
		timers.Put(timer)
		atomic.AddUint32(&p.stats.Timeouts, 1)
		return nil, ErrPoolTimeout
	}

	p.freeConnsMu.Lock()
	cn := p.popFree()
	p.freeConnsMu.Unlock()

	if cn != nil {
		atomic.AddUint32(&p.stats.Hits, 1)
		if !cn.IsStale(p.idleTimeout) {
			return cn, nil
		}
		_ = p.factory.DestroyObject(cn)
		//_ = cn.Close()
	}

	newcn, err := p.factory.MakeObject()
	if err != nil {
		p.queue <- struct{}{}
		return nil, err
	}

	p.connsMu.Lock()
	if cn != nil {
		p.remove(cn, errConnStale)
	}
	res := p.factory.ValidateObject(newcn)
	if !res {
		return nil, nil
	}
	p.conns = append(p.conns, newcn)
	p.connsMu.Unlock()
	return newcn, nil
}
func (p *ConnPool) Return(cn *PooledObject) error {
	if p.Closed() {
		err := fmt.Errorf("This pool instance has been closed")
		Logger.Print(err)
		return p.Remove(cn, err)
	}
	p.freeConnsMu.Lock()
	p.freeConns = append(p.freeConns, cn)
	p.freeConnsMu.Unlock()
	p.queue <- struct{}{}
	return nil
}
func (p *ConnPool) Remove(cn *PooledObject, reason error) error {
	_ = p.factory.DestroyObject(cn)
	p.connsMu.Lock()
	p.remove(cn, reason)
	p.connsMu.Unlock()
	p.queue <- struct{}{}
	return nil
}
func (p *ConnPool) remove(cn *PooledObject, reason error) {
	p.storeLastErr(reason.Error())
	for i, c := range p.conns {
		if c == cn {
			p.conns = append(p.conns[:i], p.conns[i+1:]...)
			break
		}
	}
}

func (p *ConnPool) storeLastErr(err string) {
	p.lastErr.Store(err)
}

// Len returns total number of connections.
func (p *ConnPool) Len() int {
	p.connsMu.Lock()
	l := len(p.conns)
	p.connsMu.Unlock()
	return l
}

// FreeLen returns number of free connections.
func (p *ConnPool) FreeLen() int {
	p.freeConnsMu.Lock()
	l := len(p.freeConns)
	p.freeConnsMu.Unlock()
	return l
}

func (p *ConnPool) Close() (retErr error) {
	if !atomic.CompareAndSwapInt32(&p._closed, 0, 1) {
		return ErrClosed
	}

	p.connsMu.Lock()

	// Close all connections.
	for _, cn := range p.conns {
		if cn == nil {
			continue
		}
		if err := p.closeConn(cn); err != nil && retErr == nil {
			retErr = err
		}
	}
	p.conns = nil
	p.connsMu.Unlock()

	p.freeConnsMu.Lock()
	p.freeConns = nil
	p.freeConnsMu.Unlock()

	return retErr
}
func (p *ConnPool) closeConn(cn *PooledObject) error {
	return p.factory.DestroyObject(cn)
}
func (p *ConnPool) ReapStaleConns() (n int, err error) {
	<-p.queue
	p.freeConnsMu.Lock()

	if len(p.freeConns) == 0 {
		p.freeConnsMu.Unlock()
		p.queue <- struct{}{}
		return
	}

	var idx int
	var cn *PooledObject
	for idx, cn = range p.freeConns {
		if !cn.IsStale(p.idleTimeout) {
			break
		}
		p.connsMu.Lock()
		p.remove(cn, errConnStale)
		p.connsMu.Unlock()
		n++
	}
	if idx == 0 {
		p.freeConns = append(p.freeConns[:idx], p.freeConns[idx+1:]...)
	}
	if idx > 0 {
		p.freeConns = append(p.freeConns[:0], p.freeConns[idx:]...)
	}

	p.freeConnsMu.Unlock()
	p.queue <- struct{}{}
	return
}

func (p *ConnPool) reaper(frequency time.Duration) {
	ticker := time.NewTicker(frequency)
	defer ticker.Stop()

	for _ = range ticker.C {
		if p.Closed() {
			break
		}
		n, err := p.ReapStaleConns()
		if err != nil {
			Logger.Printf("ReapStaleConns failed: %s", err)
			continue
		}
		s := p.Stats()
		Logger.Printf(
			"reaper: removed %d stale conns (TotalConns=%d FreeConns=%d Requests=%d Hits=%d Timeouts=%d)",
			n, s.TotalConns, s.FreeConns, s.Requests, s.Hits, s.Timeouts,
		)
	}
}
func (p *ConnPool) Stats() *PoolStats {
	stats := PoolStats{}
	stats.Requests = atomic.LoadUint32(&p.stats.Requests)
	stats.Hits = atomic.LoadUint32(&p.stats.Hits)
	stats.Waits = atomic.LoadUint32(&p.stats.Waits)
	stats.Timeouts = atomic.LoadUint32(&p.stats.Timeouts)
	stats.TotalConns = uint32(p.Len())
	stats.FreeConns = uint32(p.FreeLen())
	return &stats
}
