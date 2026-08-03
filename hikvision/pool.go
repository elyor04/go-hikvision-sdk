package hikvision

import "sync"

// scratchPool hands out reusable, fixed-capacity byte slices for calls that
// pass a caller-owned buffer into the SDK and then copy out just the written
// portion (CaptureJPEG, STDXMLConfig, ManualSnap). Without pooling, each call
// would allocate (and immediately discard, once the right-sized result copy
// is made) several MB - fine for occasional calls, but visible GC pressure
// for a caller polling these at any real frequency (e.g. CaptureJPEG across
// many channels, or ManualSnap in an ANPR hot path).
type scratchPool struct {
	size int
	pool sync.Pool
}

func newScratchPool(size int) *scratchPool {
	return &scratchPool{
		size: size,
		pool: sync.Pool{New: func() any { return make([]byte, size) }},
	}
}

// Get returns a slice of exactly the pool's configured size.
func (p *scratchPool) Get() []byte { return p.pool.Get().([]byte) }

// Put returns buf to the pool. buf must be one this pool produced via Get
// (same length) - callers must not shrink it via reslicing before returning
// it, since that would silently shrink every future Get from this pool too.
func (p *scratchPool) Put(buf []byte) {
	if len(buf) != p.size {
		return
	}
	p.pool.Put(buf)
}
