package wanda

import (
	"io"
)

type writerToReader struct {
	w *io.PipeWriter
	r *io.PipeReader
}

func newWriterToReader(writerTo io.WriterTo) *writerToReader {
	r, w := io.Pipe()
	go func() {
		if _, err := writerTo.WriteTo(w); err != nil {
			w.CloseWithError(err)
			return
		}
		w.Close()
	}()

	return &writerToReader{
		w: w,
		r: r,
	}
}

func (r *writerToReader) Read(p []byte) (int, error) {
	return r.r.Read(p)
}
