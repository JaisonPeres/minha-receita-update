package transform

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

func pathsForSource(t sourceType, dir string) ([]string, error) {
	r, err := os.ReadDir(dir)
	if err != nil {
		return []string{}, err
	}
	var ls []string
	for _, f := range r {
		if f.IsDir() || filepath.Ext(f.Name()) == ".md5" {
			continue
		}
		if strings.Contains(strings.ToLower(f.Name()), strings.ToLower(string(t))) {
			ls = append(ls, filepath.Join(dir, f.Name()))
		}
	}
	if len(ls) == 0 {
		// Tax regime files are optional (may not be available in new infrastructure)
		if t == realProfit || t == presumedProfit || t == arbitratedProfit || t == noTaxes {
			slog.Warn("tax regime file not found (continuing without it)", "source", string(t), "directory", dir)
			return []string{}, nil
		}
		return []string{}, fmt.Errorf("could not find any file matching %s in %s", string(t), dir)
	}
	return ls, nil
}

type sourceType string

const (
	venues           sourceType = "Estabelecimentos"
	motives          sourceType = "Motivos"
	base             sourceType = "Empresas"
	cities           sourceType = "Municipios"
	cnaes            sourceType = "Cnaes"
	countries        sourceType = "Paises"
	natures          sourceType = "Naturezas"
	partners         sourceType = "Socios"
	qualifications   sourceType = "Qualificacoes"
	simpleTaxes      sourceType = "Simples"
	realProfit       sourceType = "Lucro Real"
	presumedProfit   sourceType = "Lucro Presumido"
	arbitratedProfit sourceType = "Lucro Arbitrado"
	noTaxes          sourceType = "Imunes e Isentas"
)

// being accumulative means a 1-to-many relationship: one company “accumulates”
// more than one association with records from this source
func (s sourceType) isAccumulative() bool {
	return s == partners || s == realProfit || s == presumedProfit || s == arbitratedProfit || s == noTaxes
}

type source struct {
	kind    sourceType
	dir     string
	files   []string
	readers []*archivedCSVs
	total   int64
}

func (s *source) createReaders() error {
	// Skip if no files (optional source not available)
	if len(s.files) == 0 {
		return nil
	}
	s.readers = make([]*archivedCSVs, len(s.files))
	for i, p := range s.files {
		var h bool
		var sep rune
		switch s.kind {
		case realProfit:
			sep = ','
			h = true
		case presumedProfit:
			sep = ','
			h = true
		case arbitratedProfit:
			sep = ','
			h = true
		case noTaxes:
			sep = ','
			h = true
		default:
			sep = ';'
			h = false
		}
		r, err := newArchivedCSV(p, sep, h)
		if err != nil {
			return fmt.Errorf("error reading %s: %w", p, err)
		}
		s.readers[i] = r
	}
	return nil
}

func (s *source) close() error {
	// Skip if no files (optional source not available)
	if len(s.files) == 0 {
		return nil
	}
	for _, r := range s.readers {
		if err := r.close(); err != nil {
			return fmt.Errorf("error closing %s: %w", r.path, err)
		}
	}
	return nil
}

func (s *source) resetReaders() error {
	// Skip if no files (optional source not available)
	if len(s.files) == 0 {
		return nil
	}
	if err := s.close(); err != nil {
		return fmt.Errorf("error closing readers: %w", err)
	}
	if err := s.createReaders(); err != nil {
		return fmt.Errorf("error re-creating readers: %w", err)
	}
	return nil
}

func (s *source) countCSVRows(a *archivedCSVs, count chan<- int64, errs chan<- error) {
	var t int64
	for {
		_, err := a.read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errs <- err
		}
		t++
	}
	count <- t
}

func (s *source) countLines() error {
	// Skip if no files (optional source not available)
	if len(s.files) == 0 {
		s.total = 0
		return nil
	}
	count := make(chan int64)
	errs := make(chan error)
	for _, r := range s.readers {
		go s.countCSVRows(r, count, errs)
	}
	defer func() {
		if err := s.resetReaders(); err != nil {
			slog.Warn("could not close", "source", s.kind, "path", s.dir, "error", err)
		}
		close(count)
		close(errs)
	}()
	var done int
	for {
		select {
		case err := <-errs:
			return fmt.Errorf("error counting lines: %w", err)
		case n := <-count:
			s.total += n
			done++
			if done == len(s.readers) {
				return nil
			}
		}
	}
}

func (s *source) sendTo(ctx context.Context, ch chan<- []string) error {
	// Skip if no files (optional source not available)
	if len(s.files) == 0 {
		return nil
	}
	g, ctx := errgroup.WithContext(ctx)
	for _, r := range s.readers {
		a := r
		g.Go(func() error {
			if err := a.sendTo(ctx, ch); err != io.EOF {
				return err
			}
			return nil
		})
	}
	return g.Wait()
}

func newSource(ctx context.Context, t sourceType, d string) (*source, error) {
	slog.Info(fmt.Sprintf("Loading %s files…", string(t)))
	var s source
	ch := make(chan error, 1)
	done := atomic.Bool{}
	defer close(ch)
	go func() {
		ls, err := pathsForSource(t, d)
		if err != nil {
			if !done.Load() {
				ch <- fmt.Errorf("error getting files for %s in %s: %w", string(t), d, err)
			}
			return
		}
		// If no files found but it's an optional source, return empty source
		if len(ls) == 0 {
			s = source{kind: t, dir: d, files: []string{}, total: 0}
			if !done.Load() {
				ch <- nil
			}
			return
		}
		s = source{kind: t, dir: d, files: ls}
		if err := s.createReaders(); err != nil {
			ch <- fmt.Errorf("error creating readers: %w", err)
		}
		if err = s.countLines(); err != nil {
			if !done.Load() {
				ch <- fmt.Errorf("error counting lines for %s in %s: %w", string(t), d, err)
			}
		}
		if !done.Load() {
			ch <- nil
		}
	}()
	select {
	case <-ctx.Done():
		done.Store(true)
		return nil, ctx.Err()
	case err := <-ch:
		return &s, err
	}
}

func newSources(dir string, kinds []sourceType) ([]*source, int64, error) {
	srcs := []*source{}
	ok := make(chan *source)
	errs := make(chan error, 1)
	done := atomic.Bool{}
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		close(ok)
		close(errs)
	}()
	for _, s := range kinds {
		go func(s sourceType) {
			src, err := newSource(ctx, s, dir)
			if err != nil {
				if !done.Load() {
					errs <- fmt.Errorf("could not load source %s: %w", string(s), err)
				}
				return
			}
			if !done.Load() {
				ok <- src
			}
		}(s)
	}
	var t int64
	for {
		select {
		case err := <-errs:
			done.Store(true)
			return nil, 0, fmt.Errorf("error loading sources: %w", err)
		case src := <-ok:
			t += src.total
			srcs = append(srcs, src)
			slog.Info(fmt.Sprintf("[%d/%d] %s loaded!", len(srcs), len(kinds), src.kind))
			if len(srcs) == len(kinds) {
				return srcs, t, nil
			}
		}
	}
}
