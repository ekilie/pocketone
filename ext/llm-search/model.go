package llmsearch

import (
	"context"
	"strings"

	"ganigeorgiev.com/pocketbase/dbx"

	kvv1 "pocketone.dev/ext/contracts/kv/v1"
	"pocketone.dev/ext/contracts/llm"
	embedv1 "pocketone.dev/ext/contracts/llm/embed/v1"
	searchv1 "pocketone.dev/ext/contracts/llm/search/v1"
	"pocketone.dev/ext/extkit"
)

// modelKey is the kv/v1 value that holds the app's embedding model (private: Go only).
const modelKey = "_llm_search/model"

// knownModel is the app's embedding model as stored, without picking one ("" when none is).
func (s *service) knownModel() string {
	s.mu.Lock()
	m := s.model
	s.mu.Unlock()
	if m != "" {
		return m
	}
	var stored string
	if _, err := kvv1.Get(s.app, modelKey, &stored); err != nil || stored == "" {
		return ""
	}
	s.mu.Lock()
	s.model = stored
	s.mu.Unlock()
	return stored
}

// currentModel is the app's embedding model, picking one from llm.embed/v1's models the first time. It returns
// searchv1.ErrNoModel when there's none to pick.
func (s *service) currentModel(ctx context.Context) (string, error) {
	if m := s.knownModel(); m != "" {
		return m, nil
	}
	m, err := s.pickModel(ctx)
	if err != nil {
		return "", err
	}
	if err := kvv1.Set(s.app, modelKey, m); err != nil {
		return "", err
	}
	s.mu.Lock()
	s.model = m
	s.mu.Unlock()
	return m, nil
}

// pickModel chooses among the embedding models of the app's providers: the cheapest per input token (unpriced ones
// after priced ones), then by name, so the choice is the same every time.
func (s *service) pickModel(ctx context.Context) (string, error) {
	embed, ok := embedv1.Contract.Of(s.app)
	if !ok {
		return "", searchv1.ErrNoModel
	}
	list, err := embed.Models(ctx, extkit.Internal, llm.ModelsInput{})
	if err != nil || len(list.Models) == 0 {
		return "", searchv1.ErrNoModel
	}
	best, bestPrice := "", -1.0
	for _, m := range list.Models {
		price := inputPrice(m)
		better := best == "" ||
			(price >= 0 && (bestPrice < 0 || price < bestPrice)) ||
			(price == bestPrice && m.ID < best)
		if better {
			best, bestPrice = m.ID, price
		}
	}
	return best, nil
}

// inputPrice is a model's USD per 1M input tokens, or -1 when it lists none.
func inputPrice(m llm.ModelInfo) float64 {
	for _, p := range m.Prices {
		if strings.Contains(strings.ToLower(p.Unit), "token") {
			return p.USD
		}
	}
	return -1
}

// Status implements search/v1.
func (s *service) Status(ctx context.Context, _ extkit.Auth) (searchv1.Status, error) {
	st := searchv1.Status{}
	m, err := s.currentModel(ctx)
	if err != nil {
		st.Note = searchv1.ErrNoModel.Message
	}
	st.Model = m
	_ = s.app.DB().Select("count(*)").From(ChunksCollection).Row(&st.Chunks)
	if m != "" {
		_ = s.app.DB().Select("count(*)").From(ChunksCollection).Where(dbx.HashExp{"model": m}).AndWhere(dbx.NewExp("vector != ''")).Row(&st.Embedded)
	}
	return st, nil
}

// SetModel implements search/v1.
func (s *service) SetModel(ctx context.Context, a extkit.Auth, in searchv1.ModelInput) (searchv1.Status, error) {
	m := strings.TrimSpace(in.Model)
	if m == "" {
		picked, err := s.pickModel(ctx)
		if err != nil {
			return searchv1.Status{}, err
		}
		m = picked
	} else if _, _, ok := llm.SplitModel(m); !ok {
		return searchv1.Status{}, badRequest("name the model as provider/model")
	}
	if m != s.knownModel() {
		if err := kvv1.Set(s.app, modelKey, m); err != nil {
			return searchv1.Status{}, err
		}
		s.mu.Lock()
		s.model = m
		s.mu.Unlock()
		// every chunk whose vector came from another model is embedded again; keywords still find it meanwhile
		if _, err := s.app.DB().Update(DocumentsCollection, dbx.Params{"status": StatusQueued, "error": ""},
			dbx.NewExp("chunks > 0")).Execute(); err != nil {
			return searchv1.Status{}, err
		}
		s.bumpAll()
		s.kickWorker()
	}
	return s.Status(ctx, a)
}
