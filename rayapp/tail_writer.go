package rayapp

import "bytes"

// tailWriter keeps the most recent `limit` bytes written to it.
// It uses a double-buffer strategy with two bytes.Buffers: writes go
// into `active`; when active exceeds half the limit, the older buffer
// is discarded and the two are swapped. Initial memory footprint is
// near zero because bytes.Buffer grows lazily.
type tailWriter struct {
	stale  bytes.Buffer
	active bytes.Buffer
	limit  int
	half   int
}

func newTailWriter(limit int) *tailWriter {
	return &tailWriter{
		limit: limit,
		half:  limit / 2,
	}
}

func (w *tailWriter) Write(p []byte) (int, error) {
	n := len(p)
	if n == 0 {
		return 0, nil
	}

	// If a single write is >= limit, keep only the last `limit` bytes.
	if n >= w.limit {
		w.stale.Reset()
		w.active.Reset()
		w.active.Write(p[n-w.limit:])
		return n, nil
	}

	w.active.Write(p)

	if w.active.Len() > w.half {
		w.rotate()
	}
	return n, nil
}

func (w *tailWriter) rotate() {
	w.stale.Reset()
	w.stale, w.active = w.active, w.stale
}

// String returns the most recent `limit` bytes as a string.
func (w *tailWriter) String() string {
	staleBytes := w.stale.Bytes()
	activeBytes := w.active.Bytes()
	total := len(staleBytes) + len(activeBytes)

	if total <= w.limit {
		var buf bytes.Buffer
		buf.Grow(total)
		buf.Write(staleBytes)
		buf.Write(activeBytes)
		return buf.String()
	}

	// Combined exceeds limit — trim from the front of stale.
	skip := total - w.limit
	var buf bytes.Buffer
	buf.Grow(w.limit)
	buf.Write(staleBytes[skip:])
	buf.Write(activeBytes)
	return buf.String()
}
