package search

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"ganigeorgiev.com/pocketbase/dbx"

	"pocketone.dev/ext/contracts/llm"
	embedv1 "pocketone.dev/ext/contracts/llm/embed/v1"
	searchv1 "pocketone.dev/ext/contracts/search/v1"
	"pocketone.dev/ext/extkit"
)

// Search sizes.
const (
	DefaultLimit = 5
	MaxLimit     = 50
	rrfK         = 60 // reciprocal rank fusion: score = Σ 1/(rrfK + rank)
	maxQueries   = 256
)

// vecCache is a bucket's vectors as of a version.
type vecCache struct {
	version int64
	ids     []string
	names   []string // each chunk's document name, for Prefix
	vecs    [][]float32
}

type vecRow struct {
	ID     string `db:"id"`
	Name   string `db:"name"`
	Vector string `db:"vector"`
}

// vectors returns a bucket's vectors, loading them when they changed.
func (s *service) vectors(bucketID string) (*vecCache, error) {
	s.mu.Lock()
	version := s.versions[bucketID]
	c := s.vecs[bucketID]
	s.mu.Unlock()
	if c != nil && c.version == version {
		return c, nil
	}
	var rows []vecRow
	err := s.app.DB().Select("c.id AS id", "d.name AS name", "c.vector AS vector").
		From(ChunksCollection+" c").InnerJoin(DocumentsCollection+" d", dbx.NewExp("d.id = c.document")).
		Where(dbx.And(dbx.HashExp{"c.bucket": bucketID}, dbx.NewExp("c.vector != ''"))).All(&rows)
	if err != nil {
		return nil, err
	}
	c = &vecCache{version: version}
	for _, r := range rows {
		if v := decodeVector(r.Vector); len(v) > 0 {
			c.ids, c.names, c.vecs = append(c.ids, r.ID), append(c.names, r.Name), append(c.vecs, v)
		}
	}
	s.mu.Lock()
	s.vecs[bucketID] = c
	s.mu.Unlock()
	return c, nil
}

// queryVector embeds a query (cached).
func (s *service) queryVector(ctx context.Context, model string, dims int, q string) ([]float32, llm.Usage, string, error) {
	key := model + "|" + strconv.Itoa(dims) + "|" + q
	s.mu.Lock()
	v, ok := s.queries[key]
	s.mu.Unlock()
	if ok {
		return v, llm.Usage{}, model, nil
	}
	embed, ok := embedv1.Contract.Of(s.app)
	if !ok {
		return nil, llm.Usage{}, "", llm.ErrNotConfigured
	}
	res, err := embed.Embed(ctx, extkit.Internal, embedv1.EmbedInput{Model: model, Input: []string{q}, Dimensions: dims})
	if err != nil {
		return nil, llm.Usage{}, "", err
	}
	if len(res.Vectors) != 1 {
		return nil, llm.Usage{}, "", fmt.Errorf("search: the model returned %d vectors for one query", len(res.Vectors))
	}
	f := make([]float32, len(res.Vectors[0]))
	for i, x := range res.Vectors[0] {
		f[i] = float32(x)
	}
	v = normalize(f)
	s.mu.Lock()
	if len(s.queries) >= maxQueries {
		clear(s.queries)
	}
	s.queries[key] = v
	s.mu.Unlock()
	return v, res.Usage, res.Model, nil
}

type hit struct {
	id    string
	score float64
}

var wordRe = regexp.MustCompile(`[\p{L}\p{N}_]+`)

// ftsQuery is a query as FTS5 terms: each word quoted (no operators from the user), any of them matching.
func ftsQuery(q string) string {
	words := wordRe.FindAllString(q, 32)
	for i, w := range words {
		words[i] = `"` + w + `"`
	}
	return strings.Join(words, " OR ")
}

// keyword ranks a bucket's chunks by BM25.
func (s *service) keyword(bucketID, q, prefix string, n int) ([]hit, error) {
	m := ftsQuery(q)
	if m == "" {
		return nil, nil
	}
	sql := "SELECT chunk, bm25(" + FTSTable + ") AS r FROM " + FTSTable + " WHERE " + FTSTable + " MATCH {:m} AND bucket = {:b}"
	params := dbx.Params{"m": m, "b": bucketID, "n": n}
	if prefix != "" {
		sql += " AND substr(name, 1, {:pl}) = {:p}"
		params["p"], params["pl"] = prefix, len(prefix)
	}
	sql += " ORDER BY r LIMIT {:n}"
	var rows []struct {
		Chunk string  `db:"chunk"`
		R     float64 `db:"r"`
	}
	if err := s.app.DB().NewQuery(sql).Bind(params).All(&rows); err != nil {
		return nil, err
	}
	out := make([]hit, len(rows))
	for i, r := range rows {
		out[i] = hit{r.Chunk, -r.R} // bm25() is lower for better matches
	}
	return out, nil
}

// semantic ranks a bucket's chunks by cosine similarity to qv.
func semantic(c *vecCache, qv []float32, prefix string, n int) []hit {
	var out []hit
	for i, v := range c.vecs {
		if len(v) != len(qv) || (prefix != "" && !strings.HasPrefix(c.names[i], prefix)) {
			continue
		}
		var dot float32
		for j := range v {
			dot += v[j] * qv[j]
		}
		out = append(out, hit{c.ids[i], float64(dot)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].score > out[j].score })
	if len(out) > n {
		out = out[:n]
	}
	return out
}

type chunkRow struct {
	ID        string `db:"id"`
	Text      string `db:"text"`
	Heading   string `db:"heading"`
	StartLine int    `db:"start_line"`
	EndLine   int    `db:"end_line"`
	Name      string `db:"name"`
}

// Search implements search/v1.
func (s *service) Search(ctx context.Context, _ extkit.Auth, in searchv1.SearchInput) (searchv1.Results, error) {
	out := searchv1.Results{Results: []searchv1.Result{}}
	b, err := s.findBucket(in.Bucket)
	if err != nil {
		return out, err
	}
	q := strings.TrimSpace(in.Query)
	if q == "" {
		return out, badRequest("give a query")
	}
	limit := in.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	limit = min(limit, MaxLimit)
	pool := max(limit*4, 20)

	mode := in.Mode
	switch mode {
	case "":
		mode = searchv1.ModeHybrid
	case searchv1.ModeHybrid, searchv1.ModeSemantic, searchv1.ModeKeyword:
	default:
		return out, badRequest("mode is hybrid, semantic or keyword")
	}
	model := b.GetString("model")
	if model == "" {
		if mode == searchv1.ModeSemantic {
			return out, badRequest("this bucket has no embedding model; search it by keywords")
		}
		mode = searchv1.ModeKeyword
	}
	out.Mode = mode

	var kw, sem []hit
	if mode != searchv1.ModeSemantic {
		if kw, err = s.keyword(b.Id, q, in.Prefix, pool); err != nil {
			return out, err
		}
	}
	if mode != searchv1.ModeKeyword {
		qv, usage, used, err := s.queryVector(ctx, model, b.GetInt("dimensions"), q)
		switch {
		case err != nil && mode == searchv1.ModeHybrid:
			// the keyword half still answers: a page that grounds a reply in the bucket keeps working
			mode, out.Mode, out.Note = searchv1.ModeKeyword, searchv1.ModeKeyword, "ranked by keywords only: "+errText(err)
		case err != nil:
			return out, err
		default:
			out.Usage, out.Model = usage, used
			c, err := s.vectors(b.Id)
			if err != nil {
				return out, err
			}
			sem = semantic(c, qv, in.Prefix, pool)
		}
	}

	type scored struct {
		id                     string
		score, semantic, kword float64
	}
	byID := map[string]*scored{}
	get := func(id string) *scored {
		if byID[id] == nil {
			byID[id] = &scored{id: id}
		}
		return byID[id]
	}
	for rank, h := range kw {
		x := get(h.id)
		x.kword = h.score
		if mode == searchv1.ModeHybrid {
			x.score += 1 / float64(rrfK+rank+1)
		} else {
			x.score = h.score
		}
	}
	for rank, h := range sem {
		x := get(h.id)
		x.semantic = h.score
		if mode == searchv1.ModeHybrid {
			x.score += 1 / float64(rrfK+rank+1)
		} else {
			x.score = h.score
		}
	}
	ranked := make([]*scored, 0, len(byID))
	for _, x := range byID {
		ranked = append(ranked, x)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].id < ranked[j].id
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	if len(ranked) == 0 {
		return out, nil
	}

	ids := make([]any, len(ranked))
	for i, x := range ranked {
		ids[i] = x.id
	}
	var rows []chunkRow
	err = s.app.DB().Select("c.id AS id", "c.text AS text", "c.heading AS heading", "c.start_line AS start_line",
		"c.end_line AS end_line", "d.name AS name").
		From(ChunksCollection+" c").InnerJoin(DocumentsCollection+" d", dbx.NewExp("d.id = c.document")).
		Where(dbx.In("c.id", ids...)).All(&rows)
	if err != nil {
		return out, err
	}
	rowByID := map[string]chunkRow{}
	for _, r := range rows {
		rowByID[r.ID] = r
	}
	for _, x := range ranked {
		r, ok := rowByID[x.id]
		if !ok {
			continue // removed meanwhile
		}
		out.Results = append(out.Results, searchv1.Result{Text: r.Text, Document: r.Name, Heading: r.Heading,
			StartLine: r.StartLine, EndLine: r.EndLine, Score: x.score, Semantic: x.semantic, Keyword: x.kword})
	}
	return out, nil
}
