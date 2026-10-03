package engine

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"conductor/internal/block"
	"conductor/internal/run"
)

const (
	flushEvery    = 200 * time.Millisecond
	flushAtLines  = 500
	maxLineLength = 8 << 10
)

// logSink collects log lines from all steps of a run and writes them to the
// store in batches, so a chatty command doesn't mean one insert per line.
type logSink struct {
	ctx   context.Context
	runs  run.Repository
	runID string
	log   *slog.Logger

	mu    sync.Mutex
	batch []run.LogLine

	stop chan struct{}
	done chan struct{}
}

func newLogSink(ctx context.Context, runs run.Repository, runID string, log *slog.Logger) *logSink {
	s := &logSink{
		ctx:   ctx,
		runs:  runs,
		runID: runID,
		log:   log,
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
	go s.loop()
	return s
}

func (s *logSink) loop() {
	defer close(s.done)
	t := time.NewTicker(flushEvery)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			s.flush()
		case <-s.stop:
			s.flush()
			return
		}
	}
}

func (s *logSink) add(stepID string, stream run.Stream, text string) {
	if len(text) > maxLineLength {
		text = text[:maxLineLength] + " … (line truncated)"
	}
	s.mu.Lock()
	s.batch = append(s.batch, run.LogLine{StepID: stepID, Stream: stream, Text: text, Time: time.Now().UTC()})
	full := len(s.batch) >= flushAtLines
	s.mu.Unlock()
	if full {
		s.flush()
	}
}

func (s *logSink) flush() {
	s.mu.Lock()
	batch := s.batch
	s.batch = nil
	s.mu.Unlock()
	if err := s.runs.AppendLogs(s.ctx, s.runID, batch); err != nil {
		s.log.Error("write logs", "err", err, "lines", len(batch))
	}
}

// Close writes any remaining lines and stops the flusher.
func (s *logSink) Close() {
	close(s.stop)
	<-s.done
}

// envFor returns the block.Env for one step. Call flush when the step ends so
// a final line without a trailing newline isn't lost.
func (s *logSink) envFor(stepID string) (block.Env, func()) {
	stdout := &lineWriter{emit: func(l string) { s.add(stepID, run.StreamStdout, l) }}
	stderr := &lineWriter{emit: func(l string) { s.add(stepID, run.StreamStderr, l) }}
	env := block.Env{
		Stdout: stdout,
		Stderr: stderr,
		Logf: func(format string, args ...any) {
			s.add(stepID, run.StreamSystem, fmt.Sprintf(format, args...))
		},
	}
	return env, func() {
		stdout.Flush()
		stderr.Flush()
	}
}

// lineWriter splits a byte stream into lines. Safe for concurrent use, since
// exec may write stdout and stderr from separate goroutines.
type lineWriter struct {
	mu   sync.Mutex
	buf  []byte
	emit func(string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		w.emit(string(bytes.TrimRight(w.buf[:i], "\r")))
		w.buf = w.buf[i+1:]
	}
	// A huge line without newlines would otherwise grow without bound.
	if len(w.buf) > maxLineLength {
		w.emit(string(w.buf))
		w.buf = w.buf[:0]
	}
	return len(p), nil
}

func (w *lineWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) > 0 {
		w.emit(string(w.buf))
		w.buf = nil
	}
}
