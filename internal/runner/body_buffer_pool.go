package runner

import (
	"bytes"
	"sync"
)

// bodyBufPool recycles *bytes.Buffer instances used to read HTTP response
// bodies. This eliminates the repeated heap allocations that io.ReadAll
// performs when it grows its internal slice, significantly reducing GC
// pressure during high-throughput load tests.

const initialBufferSize = 8 * 1024

const maxPooledBuffCap = 1024*1024

var bodyBufPool = sync.Pool{
	New: func() any { return bytes.NewBuffer(make([]byte, 0, initialBufferSize)) },
}

// acquireBodyBuf returns a zeroed buffer from the pool.
func acquireBodyBuf() *bytes.Buffer {
	buf := bodyBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// releaseBodyBuf resets and returns buf to the pool.
func releaseBodyBuf(buf *bytes.Buffer) {
	if buf.Cap() > maxPooledBuffCap {
		return
	}
	
	buf.Reset()
	bodyBufPool.Put(buf)
}
