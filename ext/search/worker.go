package search

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"math"
	"time"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"pocketone.dev/ext/contracts/llm"
	embedv1 "pocketone.dev/ext/contracts/llm/embed/v1"
	"pocketone.dev/ext/extkit"
)

// Embedding pace: chunks per request, and the least time between requests (Vultr allows 120 a minute).
var (
	BatchSize       = 64
	RequestInterval = 550 * time.Millisecond
	retryWait       = 3 * time.Second
	// background: false leaves queued documents for drain (tests).
	background = true
)

// kickWorker starts the worker if it isn't running and tells it there's work.
func (s *service) kickWorker() {
	if !background {
		return
	}
	s.workerOnce.Do(func() { go s.worker() })
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

func (s *service) stopWorker() { s.stopOnce.Do(func() { close(s.stop) }) }

func (s *service) worker() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { <-s.stop; cancel() }()
	for {
		select {
		case <-s.stop:
			return
		case <-s.kick:
		case <-time.After(time.Minute):
		}
		_ = s.drain(ctx)
	}
}

// drain embeds queued documents until none are left (or ctx ends).
func (s *service) drain(ctx context.Context) error {
	for ctx.Err() == nil {
		more, err := s.embedNext(ctx)
		if err != nil {
			return err
		}
		if !more {
			return nil
		}
	}
	return ctx.Err()
}

type pendingChunk struct {
	ID      string `db:"id"`
	Heading string `db:"heading"`
	Text    string `db:"text"`
}

// embedNext embeds the oldest queued document; false when there's none.
//
// It holds s.work only while it writes, never across a model call, so adding or removing documents (page code, on the
// app's one JS VM) never waits for the network. A vector written for a chunk replaced meanwhile updates no row.
func (s *service) embedNext(ctx context.Context) (bool, error) {
	s.drainMu.Lock()
	defer s.drainMu.Unlock()
	docs, err := s.app.FindRecordsByFilter(DocumentsCollection, "status = {:q} || status = {:e}", "updated", 1, 0,
		dbx.Params{"q": StatusQueued, "e": StatusEmbedding})
	if err != nil || len(docs) == 0 {
		return false, err
	}
	doc := docs[0]
	b, err := s.app.FindRecordById(BucketsCollection, doc.GetString("bucket"))
	if err != nil {
		return true, s.fail(doc, "its bucket is gone")
	}
	model := b.GetString("model")
	if model == "" {
		return true, s.setStatus(doc, StatusReady, "")
	}
	embed, ok := embedv1.Contract.Of(s.app)
	if !ok {
		return true, s.fail(doc, "nothing provides llm.embed/v1 (add the llm extension and a provider in Settings → LLM)")
	}
	if err := s.setStatus(doc, StatusEmbedding, ""); err != nil {
		return true, err
	}
	name := doc.GetString("name")
	for {
		var batch []pendingChunk
		if err := s.app.DB().Select("id", "heading", "text").From(ChunksCollection).
			Where(dbx.And(dbx.HashExp{"document": doc.Id, "vector": ""})).OrderBy("seq").Limit(int64(BatchSize)).All(&batch); err != nil {
			return true, err
		}
		if len(batch) == 0 {
			break
		}
		in := embedv1.EmbedInput{Model: model, Dimensions: b.GetInt("dimensions")}
		for _, c := range batch {
			in.Input = append(in.Input, embedText(name, piece{Heading: c.Heading, Text: c.Text}))
		}
		res, err := s.embedPaced(ctx, embed, in)
		if err != nil {
			if ctx.Err() != nil {
				return false, ctx.Err() // shutting down: stays queued
			}
			return true, s.fail(doc, errText(err))
		}
		if len(res.Vectors) != len(batch) {
			return true, s.fail(doc, "the model returned a different number of vectors")
		}
		s.work.Lock()
		err = s.app.RunInTransaction(func(tx core.App) error {
			for i, c := range batch {
				if _, err := tx.DB().Update(ChunksCollection, dbx.Params{"vector": encodeVector(res.Vectors[i])}, dbx.HashExp{"id": c.ID}).Execute(); err != nil {
					return err
				}
			}
			_, err := tx.DB().NewQuery("UPDATE " + DocumentsCollection + " SET cost = cost + {:c} WHERE id = {:id}").
				Bind(dbx.Params{"c": res.Usage.CostMicroUSD, "id": doc.Id}).Execute()
			return err
		})
		s.work.Unlock()
		if err != nil {
			return true, err
		}
		s.bump(b.Id)
	}
	// ready unless the document was replaced meanwhile with chunks still to embed (then it's queued again)
	s.work.Lock()
	defer s.work.Unlock()
	var left int
	_ = s.app.DB().Select("count(*)").From(ChunksCollection).Where(dbx.HashExp{"document": doc.Id, "vector": ""}).Row(&left)
	if left > 0 {
		return true, nil
	}
	_, err = s.app.DB().Update(DocumentsCollection, dbx.Params{"status": StatusReady, "error": ""},
		dbx.HashExp{"id": doc.Id, "status": StatusEmbedding}).Execute()
	return true, err
}

// embedPaced calls the model no faster than RequestInterval, retrying once after a failure.
func (s *service) embedPaced(ctx context.Context, embed embedv1.Embed, in embedv1.EmbedInput) (embedv1.Result, error) {
	var res embedv1.Result
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		if wait := RequestInterval - time.Since(s.lastRequest); wait > 0 {
			select {
			case <-ctx.Done():
				return res, ctx.Err()
			case <-time.After(wait):
			}
		}
		s.lastRequest = time.Now()
		res, err = embed.Embed(ctx, extkit.Internal, in)
		if err == nil || isPermanent(err) {
			return res, err
		}
		select {
		case <-ctx.Done():
			return res, ctx.Err()
		case <-time.After(retryWait):
		}
	}
	return res, err
}

// isPermanent: errors a retry won't fix.
func isPermanent(err error) bool {
	var e *extkit.Error
	if errors.As(err, &e) {
		switch e.Code {
		case llm.ErrNotConfigured.Code, llm.ErrUnknownModel.Code, llm.ErrNotSupported.Code, "bad_request", "forbidden":
			return true
		}
	}
	return false
}

func errText(err error) string {
	var e *extkit.Error
	if errors.As(err, &e) {
		return e.Message
	}
	return err.Error()
}

func (s *service) setStatus(doc *core.Record, status, msg string) error {
	_, err := s.app.DB().Update(DocumentsCollection, dbx.Params{"status": status, "error": msg}, dbx.HashExp{"id": doc.Id}).Execute()
	return err
}

func (s *service) fail(doc *core.Record, msg string) error {
	if len(msg) > 2000 {
		msg = msg[:2000]
	}
	s.app.Logger().Warn("search: embedding failed", "document", doc.GetString("name"), "error", msg)
	return s.setStatus(doc, StatusError, msg)
}

// encodeVector packs a vector as little-endian float32s in base64.
func encodeVector(v []float64) string {
	b := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(float32(f)))
	}
	return base64.StdEncoding.EncodeToString(b)
}

// decodeVector unpacks a vector and scales it to length 1, so a dot product is the cosine.
func decodeVector(s string) []float32 {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(b)%4 != 0 {
		return nil
	}
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return normalize(v)
}

func normalize(v []float32) []float32 {
	var sum float64
	for _, f := range v {
		sum += float64(f) * float64(f)
	}
	if sum == 0 {
		return v
	}
	n := float32(1 / math.Sqrt(sum))
	for i := range v {
		v[i] *= n
	}
	return v
}
