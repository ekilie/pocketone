package oai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// Job is an async generation (video on OpenRouter and Vultr, music on Vultr): POST {path} answers 202 with a job, GET
// {path}/{id} polls it, GET {path}/{id}/content?index=N downloads an output. Status: pending, queued, in_progress,
// completed, failed, cancelled, expired.
type Job struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Progress int    `json:"progress"`
	// Outputs describe the files a job made, when the provider lists them (Vultr's music).
	Outputs []JobOutput `json:"outputs"`
	// URLs are signed download links when the provider gives them (OpenRouter).
	URLs      []string        `json:"urls"`
	Usage     json.RawMessage `json:"usage"`
	Error     any             `json:"error"`
	ErrorCode string          `json:"error_code"`
	Raw       json.RawMessage `json:"-"`
}

// JobOutput is one file a job made.
type JobOutput struct {
	Index       int     `json:"index"`
	Kind        string  `json:"kind"`
	ContentType string  `json:"content_type"`
	Format      string  `json:"format"`
	Duration    float64 `json:"duration_seconds"`
}

// Done reports whether the job has stopped, finished or not.
func (j Job) Done() bool {
	switch j.Status {
	case "completed", "failed", "cancelled", "expired":
		return true
	}
	return false
}

// Err is the job's failure, if it failed.
func (j Job) Err() error {
	if j.Status == "completed" || !j.Done() {
		return nil
	}
	msg := "the job " + j.Status
	if j.Error != nil && j.Error != "" {
		msg = fmt.Sprintf("%s: %v", msg, j.Error)
	}
	return &Error{Code: j.ErrorCode, Message: msg}
}

func (c *Client) job(ctx context.Context, method, path string, in any) (Job, error) {
	var raw json.RawMessage
	if err := c.JSON(ctx, method, path, in, &raw); err != nil {
		return Job{}, err
	}
	var j Job
	if err := json.Unmarshal(raw, &j); err != nil || j.ID == "" {
		return Job{}, &Error{Message: "the answer isn't a job", Body: clip(string(raw), 300)}
	}
	j.Raw = raw
	return j, nil
}

// StartJob posts req (with extra merged in) to path and returns the job.
func (c *Client) StartJob(ctx context.Context, path string, req any, extra map[string]any) (Job, error) {
	b, err := withExtra(req, extra)
	if err != nil {
		return Job{}, err
	}
	return c.job(ctx, "POST", path, json.RawMessage(b))
}

// GetJob reads a job's state.
func (c *Client) GetJob(ctx context.Context, path, id string) (Job, error) {
	return c.job(ctx, "GET", path+"/"+url.PathEscape(id), nil)
}

// JobContent downloads output index of a finished job.
func (c *Client) JobContent(ctx context.Context, path, id string, index int) (File, error) {
	res, err := c.Request(ctx, "GET", path+"/"+url.PathEscape(id)+"/content?index="+strconv.Itoa(index), nil, "")
	if err != nil {
		return File{}, err
	}
	defer res.Body.Close()
	return ReadFile(res)
}

// WaitJob polls a job every interval (default 2 s) until it stops or ctx ends; a failed job is an error.
func (c *Client) WaitJob(ctx context.Context, path string, j Job, interval time.Duration) (Job, error) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	for !j.Done() {
		select {
		case <-ctx.Done():
			return j, ctx.Err()
		case <-time.After(interval):
		}
		var err error
		if j, err = c.GetJob(ctx, path, j.ID); err != nil {
			return j, err
		}
	}
	return j, j.Err()
}
