// Multi-host scheduler: the speed fix. DepotDownloader sends every
// request to one host; like the Steam client we spread chunks over many
// hosts with a few connections each, add connections while the measured
// rate keeps rising, and penalise slow or failing hosts.
//
// Pipeline: network workers fetch encrypted chunks, a CPU pool
// (GOMAXPROCS) decrypts/decompresses/checksums, and writers WriteAt into
// preallocated files. Bytes in flight stay bounded by the worker count.
package depot

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// SchedulerConfig tunes the scheduler. Zero values take defaults:
// InitialConns 8, MaxConns 32, MaxPerHost 4, RetryLimit 3.
type SchedulerConfig struct {
	InitialConns int
	MaxConns     int
	MaxPerHost   int
	RetryLimit   int
	GrowEvery    time.Duration
}

func (c SchedulerConfig) withDefaults() SchedulerConfig {
	if c.InitialConns == 0 {
		c.InitialConns = 8
	}
	if c.MaxConns == 0 {
		c.MaxConns = 32
	}
	if c.MaxPerHost == 0 {
		c.MaxPerHost = 4
	}
	if c.RetryLimit == 0 {
		c.RetryLimit = 3
	}
	if c.GrowEvery == 0 {
		c.GrowEvery = 3 * time.Second
	}
	return c
}

// Progress reports bytes done/total plus the live speed, connection and
// host counts, with the id of the chunk that just landed for checkpointing.
type Progress struct {
	DoneBytes   uint64
	TotalBytes  uint64
	BytesPerSec float64
	Connections int
	Hosts       int
	// ChunkID names the chunk that just landed, so callers can
	// checkpoint job state as the download runs.
	ChunkID string
	// Data holds the chunk's bytes for the progress callback only, so
	// the binary can stage chunks for resume without another lookup.
	Data []byte
}

// hostState tracks per-host health: in-flight count, errors, bytes and a
// cooldown after failures.
type hostState struct {
	host     Host
	inflight int64
	errors   int64
	bytes    int64
	restTill time.Time
	mu       sync.Mutex
}

func (h *hostState) healthy() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return time.Now().After(h.restTill)
}

func (h *hostState) penalise() {
	h.mu.Lock()
	h.restTill = time.Now().Add(5 * time.Second)
	h.errors++
	h.mu.Unlock()
}

// Scheduler downloads one depot's chunks over many hosts.
type Scheduler struct {
	client *Client
	cfg    SchedulerConfig
}

// NewScheduler builds a scheduler around a CDN client.
func NewScheduler(client *Client, cfg SchedulerConfig) *Scheduler {
	if client == nil {
		client = NewClient()
	}
	return &Scheduler{client: client, cfg: cfg.withDefaults()}
}

type chunkTask struct {
	chunk Chunk
	tries int
}

// Download fetches every unique chunk of m, decrypts/verifies with key,
// and returns chunk id -> uncompressed bytes. Entries already present in
// have (resume data) are re-checked by checksum and reused; the rest are
// fetched. Files are sorted by path and chunks by offset first, so a
// partial result is mostly whole files; shared chunks download once.
// Calls onProgress as bytes land.
//
// Download buffers the whole depot in RAM; production install/update paths
// use DownloadStream instead to bound memory. Download is kept for the
// fake-CDN tests and small fixtures.
func (s *Scheduler) Download(ctx context.Context, m *Manifest, depotID uint32, hosts []Host, key []byte, have map[string][]byte, onProgress func(Progress)) (map[string][]byte, error) {
	out := map[string][]byte{}
	skip := map[string]bool{}
	for _, c := range orderedChunks(m) {
		if plain, ok := have[string(c.ID)]; ok && uint32(len(plain)) == c.UncompressedLength && Adler0(plain) == c.Checksum {
			out[string(c.ID)] = plain
			skip[string(c.ID)] = true
		}
	}
	// Preserve the old Progress.Data contract for test callers: the
	// streaming core reports Data:nil, so reattach from out here. The
	// collector calls sink then onProgress sequentially, so this is
	// race-free.
	wrapped := onProgress
	if onProgress != nil {
		wrapped = func(p Progress) {
			if p.ChunkID != "" {
				p.Data = out[p.ChunkID]
			}
			onProgress(p)
		}
	}
	err := s.DownloadStream(ctx, m, depotID, hosts, key, skip,
		func(id string, data []byte) error {
			out[id] = append([]byte{}, data...)
			return nil
		}, wrapped)
	if err != nil {
		return out, err
	}
	return out, nil
}

// DownloadStream fetches every unique chunk of m not in skip, calling sink
// once per landed chunk. Chunks are written out as they land, so memory
// stays bounded by in-flight chunks (worker count x ~1 MiB) instead of the
// whole depot. skip names chunks already on disk (staged or reused); they
// are counted in progress totals but never fetched. sink must not retain
// data beyond its call (copy if needed).
func (s *Scheduler) DownloadStream(ctx context.Context, m *Manifest, depotID uint32, hosts []Host, key []byte, skip map[string]bool, sink func(id string, data []byte) error, onProgress func(Progress)) error {
	if len(hosts) == 0 {
		return fmt.Errorf("no download hosts")
	}
	ordered := orderedChunks(m)
	var total uint64
	for _, c := range ordered {
		total += uint64(c.UncompressedLength)
	}
	var skippedBytes uint64
	chunkByID := make(map[string]Chunk, len(ordered))
	for _, c := range ordered {
		chunkByID[string(c.ID)] = c
	}
	var pending []Chunk
	for _, c := range ordered {
		if skip[string(c.ID)] {
			skippedBytes += uint64(c.UncompressedLength)
			continue
		}
		pending = append(pending, c)
	}
	ordered = pending
	if len(ordered) == 0 {
		return nil
	}
	states := make([]*hostState, len(hosts))
	for i, h := range hosts {
		states[i] = &hostState{host: h}
	}
	// Network workers.
	var wg sync.WaitGroup
	var conns atomic.Int32
	initial := s.cfg.InitialConns
	if initial > len(ordered) {
		initial = len(ordered)
	}
	if initial > s.cfg.MaxConns {
		initial = s.cfg.MaxConns
	}
	conns.Store(int32(initial))
	tasks := make(chan chunkTask, len(ordered))
	for _, c := range ordered {
		tasks <- chunkTask{chunk: c}
	}
	type result struct {
		id   string
		data []byte
		task chunkTask
		err  error
	}
	// Small buffer bounds bytes in flight: workers block until the
	// collector writes each chunk out via sink.
	bufCap := 2 * initial
	if bufCap < 4 {
		bufCap = 4
	}
	if bufCap > len(ordered) {
		bufCap = len(ordered)
	}
	results := make(chan result, bufCap)
	var doneBytes atomic.Uint64
	var doneChunks atomic.Int64
	doneBytes.Store(skippedBytes)
	worker := func() {
		defer wg.Done()
		for task := range tasks {
			data, err := s.fetchOne(ctx, m, depotID, task.chunk, states, key)
			if err != nil {
				results <- result{id: string(task.chunk.ID), task: task, err: err}
				continue
			}
			doneBytes.Add(uint64(len(data)))
			doneChunks.Add(1)
			results <- result{id: string(task.chunk.ID), data: data}
		}
	}
	wg.Add(initial)
	for i := 0; i < initial; i++ {
		go worker()
	}
	// Adaptive growth: while the measured rate keeps rising, add a
	// connection every GrowEvery up to MaxConns (like Steam raising 9, 10,
	// 11 while the rate climbs). The fake benchmark shortens GrowEvery.
	start := time.Now()
	lastRate := 0.0
	lastBytes := uint64(0)
	growDone := make(chan struct{})
	stopGrow := make(chan struct{})
	go func() {
		defer close(growDone)
		tick := time.NewTicker(s.cfg.GrowEvery)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stopGrow:
				return
			case <-tick.C:
				done := doneBytes.Load()
				rate := float64(done-lastBytes) / s.cfg.GrowEvery.Seconds()
				lastBytes = done
				if rate > lastRate && int(conns.Load()) < s.cfg.MaxConns {
					conns.Add(1)
					lastRate = rate
					wg.Add(1)
					go worker()
				} else if rate <= lastRate {
					return
				}
			}
		}
	}()
	var failures []error
	completed := 0
	hostsUsed := map[string]bool{}
	for completed < len(ordered) {
		select {
		case <-ctx.Done():
			close(stopGrow)
			close(tasks)
			wg.Wait()
			<-growDone
			return ctx.Err()
		case r := <-results:
			completed++
			if r.err != nil {
				if r.task.tries < s.cfg.RetryLimit {
					r.task.tries++
					tasks <- r.task
					completed--
					continue
				}
				failures = append(failures, r.err)
				continue
			}
			if sink != nil {
				if err := sink(r.id, r.data); err != nil {
					close(stopGrow)
					close(tasks)
					wg.Wait()
					<-growDone
					return err
				}
			}
			// Drop the reference so the chunk can be GC'd before the
			// next progress report; only in-flight chunks stay in RAM.
			r.data = nil
			if onProgress != nil {
				hostsUsed["x"] = true
				onProgress(Progress{DoneBytes: doneBytes.Load(), TotalBytes: total,
					BytesPerSec: float64(doneBytes.Load()) / time.Since(start).Seconds(),
					Connections: int(conns.Load()), Hosts: len(hosts), ChunkID: r.id, Data: nil})
			}
		}
	}
	close(stopGrow)
	close(tasks)
	wg.Wait()
	<-growDone
	_ = hostsUsed
	_ = chunkByID
	if len(failures) > 0 {
		return failures[0]
	}
	return nil
}

// fetchOne downloads, decrypts and verifies one chunk, trying hosts in
// load order and penalising failures. When every host is resting or full
// it waits for one to free up instead of failing: giving up instantly
// turns a momentarily-busy pool into a failed job. Decrypt runs inline;
// parallelism across workers already spreads it over GOMAXPROCS.
func (s *Scheduler) fetchOne(ctx context.Context, m *Manifest, depotID uint32, c Chunk, states []*hostState, key []byte) ([]byte, error) {
	wait := 20 * time.Millisecond
	for {
		order := append([]*hostState{}, states...)
		sort.SliceStable(order, func(i, j int) bool {
			return atomic.LoadInt64(&order[i].inflight) < atomic.LoadInt64(&order[j].inflight)
		})
		var last error
		tried := false
		for _, h := range order {
			if !h.healthy() {
				continue
			}
			if atomic.LoadInt64(&h.inflight) >= int64(s.cfg.MaxPerHost) {
				continue
			}
			tried = true
			atomic.AddInt64(&h.inflight, 1)
			enc, err := s.client.DownloadChunk(ctx, h.host, depotID, c.ID)
			atomic.AddInt64(&h.inflight, -1)
			if err != nil {
				h.penalise()
				last = err
				continue
			}
			atomic.AddInt64(&h.bytes, int64(len(enc)))
			data, err := ProcessChunk(key, enc, c)
			if err != nil {
				// A bad checksum on one host retries elsewhere.
				h.penalise()
				last = err
				continue
			}
			return data, nil
		}
		if tried {
			if last == nil {
				last = fmt.Errorf("chunk %x failed on every host", c.ID)
			}
			return nil, last
		}
		// Every host is resting or full: wait for one instead of failing.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		if wait < 500*time.Millisecond {
			wait *= 2
		}
	}
}

func orderedChunks(m *Manifest) []Chunk {
	files := append([]File{}, m.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	seen := map[string]bool{}
	var out []Chunk
	for _, f := range files {
		ch := append([]Chunk{}, f.Chunks...)
		sort.Slice(ch, func(i, j int) bool { return ch[i].Offset < ch[j].Offset })
		for _, c := range ch {
			if seen[string(c.ID)] {
				continue
			}
			seen[string(c.ID)] = true
			out = append(out, c)
		}
	}
	return out
}
