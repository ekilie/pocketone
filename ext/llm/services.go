package llm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	llmc "pocketone.dev/ext/contracts/llm"
	embedv1 "pocketone.dev/ext/contracts/llm/embed/v1"
	imagev1 "pocketone.dev/ext/contracts/llm/image/v1"
	musicv1 "pocketone.dev/ext/contracts/llm/music/v1"
	speechv1 "pocketone.dev/ext/contracts/llm/speech/v1"
	textv1 "pocketone.dev/ext/contracts/llm/text/v1"
	transcribev1 "pocketone.dev/ext/contracts/llm/transcribe/v1"
	videov1 "pocketone.dev/ext/contracts/llm/video/v1"
	"pocketone.dev/ext/extkit"
)

// One service per contract, each over the router.

func badInput(msg string) error { return &extkit.Error{Code: "bad_request", Status: 400, Message: msg} }

// errStopped: a stream's receiver said stop.
var errStopped = errors.New("stopped")

type textService struct{ r *router }

var _ textv1.Text = textService{}

func checkText(in textv1.CompleteInput) error {
	if strings.TrimSpace(in.Prompt) == "" && len(in.Messages) == 0 {
		return badInput("give a prompt or messages")
	}
	if len(in.Schema) > 0 && !json.Valid(in.Schema) {
		return badInput("schema must be a JSON schema")
	}
	switch in.Reasoning {
	case "", "off", "low", "medium", "high":
	default:
		return badInput(`reasoning must be "", "off", "low", "medium" or "high"`)
	}
	return nil
}

func (s textService) answer(ctx context.Context, in textv1.CompleteInput, onDelta func(text, reasoning string) bool) (textv1.CompleteResult, error) {
	if err := checkText(in); err != nil {
		return textv1.CompleteResult{}, err
	}
	t, err := s.r.resolve(llmc.KindText, in.Model)
	if err != nil {
		return textv1.CompleteResult{}, err
	}
	res, err := t.d.complete(ctx, t.model, in, onDelta)
	res.Model = t.ref()
	if err != nil && !errors.Is(err, errStopped) {
		return res, providerError(t.p, err)
	}
	if errors.Is(err, errStopped) {
		res.StopReason = "cancelled"
	}
	return res, nil
}

func (s textService) Complete(ctx context.Context, _ extkit.Auth, in textv1.CompleteInput) (textv1.CompleteResult, error) {
	return s.answer(ctx, in, nil)
}

func (s textService) Stream(_ context.Context, _ extkit.Auth, in textv1.CompleteInput) (extkit.Stream, error) {
	if err := checkText(in); err != nil {
		return extkit.Stream{}, err
	}
	if _, err := s.r.resolve(llmc.KindText, in.Model); err != nil {
		return extkit.Stream{}, err // before the stream starts, as a plain error
	}
	return extkit.Stream{Run: func(ctx context.Context, send func(any) error) error {
		res, err := s.answer(ctx, in, func(text, reasoning string) bool {
			if text == "" && reasoning == "" {
				return true
			}
			return send(textv1.Chunk{Text: text, Reasoning: reasoning}) == nil
		})
		if err != nil {
			return err
		}
		_ = send(textv1.Chunk{Done: true, Result: &res})
		return nil
	}}, nil
}

// Count estimates: about four characters a token (no provider here counts without charging).
func (s textService) Count(_ context.Context, _ extkit.Auth, in textv1.CountInput) (textv1.CountResult, error) {
	n := utf8.RuneCountInString(in.Text) + utf8.RuneCountInString(in.System)
	for _, m := range in.Messages {
		n += utf8.RuneCountInString(m.Content) + 4
	}
	return textv1.CountResult{Tokens: (n + 3) / 4}, nil
}

func (s textService) Models(ctx context.Context, _ extkit.Auth, in llmc.ModelsInput) (llmc.Models, error) {
	return s.r.models(ctx, llmc.KindText, in)
}

type imageService struct{ r *router }

var _ imagev1.Image = imageService{}

func (s imageService) run(ctx context.Context, in imagev1.GenerateInput, edit []llmc.File) (imagev1.Result, error) {
	if strings.TrimSpace(in.Prompt) == "" {
		return imagev1.Result{}, badInput("give a prompt")
	}
	t, err := s.r.resolve(llmc.KindImage, in.Model)
	if err != nil {
		return imagev1.Result{}, err
	}
	res, err := t.d.image(ctx, t.model, in, edit)
	res.Model = t.ref()
	if res.Files == nil {
		res.Files = []llmc.File{}
	}
	return res, providerError(t.p, err)
}

func (s imageService) Generate(ctx context.Context, _ extkit.Auth, in imagev1.GenerateInput) (imagev1.Result, error) {
	return s.run(ctx, in, nil)
}

func (s imageService) Edit(ctx context.Context, _ extkit.Auth, in imagev1.EditInput) (imagev1.Result, error) {
	if len(in.Images) == 0 {
		return imagev1.Result{}, badInput("give the images to edit")
	}
	return s.run(ctx, in.GenerateInput, in.Images)
}

func (s imageService) Models(ctx context.Context, _ extkit.Auth, in llmc.ModelsInput) (llmc.Models, error) {
	return s.r.models(ctx, llmc.KindImage, in)
}

type speechService struct{ r *router }

var _ speechv1.Speech = speechService{}

func (s speechService) Speak(ctx context.Context, _ extkit.Auth, in speechv1.SpeakInput) (speechv1.Result, error) {
	if strings.TrimSpace(in.Text) == "" {
		return speechv1.Result{}, badInput("give the text to say")
	}
	t, err := s.r.resolve(llmc.KindSpeech, in.Model)
	if err != nil {
		return speechv1.Result{}, err
	}
	res, err := t.d.speak(ctx, t.model, in)
	res.Model = t.ref()
	return res, providerError(t.p, err)
}

func (s speechService) Voices(ctx context.Context, _ extkit.Auth, in speechv1.VoicesInput) (speechv1.VoiceList, error) {
	t, err := s.r.resolve(llmc.KindSpeech, in.Model)
	if err != nil {
		return speechv1.VoiceList{}, err
	}
	vs, err := t.d.voices(ctx, t.model)
	if vs == nil {
		vs = []speechv1.Voice{}
	}
	return speechv1.VoiceList{Voices: vs}, providerError(t.p, err)
}

func (s speechService) Models(ctx context.Context, _ extkit.Auth, in llmc.ModelsInput) (llmc.Models, error) {
	return s.r.models(ctx, llmc.KindSpeech, in)
}

type transcribeService struct{ r *router }

var _ transcribev1.Transcribe = transcribeService{}

func (s transcribeService) Transcribe(ctx context.Context, _ extkit.Auth, in transcribev1.TranscribeInput) (transcribev1.Result, error) {
	if len(in.Audio.Data) == 0 {
		return transcribev1.Result{}, badInput("give the audio's data")
	}
	t, err := s.r.resolve(llmc.KindTranscribe, in.Model)
	if err != nil {
		return transcribev1.Result{}, err
	}
	res, err := t.d.transcribe(ctx, t.model, in)
	res.Model = t.ref()
	return res, providerError(t.p, err)
}

func (s transcribeService) Models(ctx context.Context, _ extkit.Auth, in llmc.ModelsInput) (llmc.Models, error) {
	return s.r.models(ctx, llmc.KindTranscribe, in)
}

type embedService struct{ r *router }

var _ embedv1.Embed = embedService{}

func (s embedService) Embed(ctx context.Context, _ extkit.Auth, in embedv1.EmbedInput) (embedv1.Result, error) {
	if len(in.Input) == 0 {
		return embedv1.Result{}, badInput("give the texts to embed")
	}
	t, err := s.r.resolve(llmc.KindEmbed, in.Model)
	if err != nil {
		return embedv1.Result{}, err
	}
	res, err := t.d.embed(ctx, t.model, in)
	res.Model = t.ref()
	if res.Vectors == nil {
		res.Vectors = [][]float64{}
	}
	return res, providerError(t.p, err)
}

func (s embedService) Models(ctx context.Context, _ extkit.Auth, in llmc.ModelsInput) (llmc.Models, error) {
	return s.r.models(ctx, llmc.KindEmbed, in)
}

// jobService is music and video: the same job operations for kind.
type jobService struct {
	r    *router
	kind string
}

func (s jobService) create(ctx context.Context, model, wait string, in jobInput) (llmc.Job, error) {
	if strings.TrimSpace(in.Prompt) == "" {
		return llmc.Job{}, badInput("give a prompt")
	}
	t, err := s.r.resolve(s.kind, model)
	if err != nil {
		return llmc.Job{}, err
	}
	j, err := t.d.startJob(ctx, s.kind, t.model, in)
	if err != nil {
		return llmc.Job{}, providerError(t.p, err)
	}
	j.ID, j.Model = jobID(t.p, j.ID), t.ref()
	return s.r.waitJob(ctx, t, s.kind, j, wait)
}

func (s jobService) get(ctx context.Context, in llmc.JobRef) (llmc.Job, error) {
	t, pid, err := s.r.job(in.ID)
	if err != nil {
		return llmc.Job{}, err
	}
	j, err := t.d.getJob(ctx, s.kind, pid)
	if err != nil {
		return llmc.Job{}, providerError(t.p, err)
	}
	j.ID = in.ID
	return j, nil
}

func (s jobService) content(ctx context.Context, in llmc.ContentInput) (llmc.File, error) {
	t, pid, err := s.r.job(in.ID)
	if err != nil {
		return llmc.File{}, err
	}
	f, err := t.d.jobContent(ctx, s.kind, pid, in.Index)
	return f, providerError(t.p, err)
}

type musicService struct{ jobService }

var _ musicv1.Music = musicService{}

func (s musicService) Create(ctx context.Context, _ extkit.Auth, in musicv1.CreateInput) (llmc.Job, error) {
	return s.create(ctx, in.Model, in.Wait, jobInput{Prompt: in.Prompt, Lyrics: in.Lyrics, Seconds: in.Seconds, Options: in.Options})
}
func (s musicService) Get(ctx context.Context, _ extkit.Auth, in llmc.JobRef) (llmc.Job, error) {
	return s.get(ctx, in)
}
func (s musicService) Content(ctx context.Context, _ extkit.Auth, in llmc.ContentInput) (llmc.File, error) {
	return s.content(ctx, in)
}
func (s musicService) Models(ctx context.Context, _ extkit.Auth, in llmc.ModelsInput) (llmc.Models, error) {
	return s.r.models(ctx, llmc.KindMusic, in)
}

type videoService struct{ jobService }

var _ videov1.Video = videoService{}

func (s videoService) Create(ctx context.Context, _ extkit.Auth, in videov1.CreateInput) (llmc.Job, error) {
	return s.create(ctx, in.Model, in.Wait, jobInput{Prompt: in.Prompt, Seconds: in.Seconds, Size: in.Size, AspectRatio: in.AspectRatio,
		Image: in.Image, Options: in.Options})
}
func (s videoService) Get(ctx context.Context, _ extkit.Auth, in llmc.JobRef) (llmc.Job, error) {
	return s.get(ctx, in)
}
func (s videoService) Content(ctx context.Context, _ extkit.Auth, in llmc.ContentInput) (llmc.File, error) {
	return s.content(ctx, in)
}
func (s videoService) Models(ctx context.Context, _ extkit.Auth, in llmc.ModelsInput) (llmc.Models, error) {
	return s.r.models(ctx, llmc.KindVideo, in)
}
