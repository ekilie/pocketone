package search

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"

	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"pocketone.dev/ext/extkit"
)

// Repository limits.
var (
	GitTimeout       = 5 * time.Minute
	DefaultCommits   = 300  // recent commits that become documents
	MaxCommits       = 2000 // at most
	MaxRepoFiles     = 5000
	commitFilesShown = 50
)

// readGit reads a repository source: its default branch's text files and its recent commits (one document each:
// subject, body, author, date, files changed; no diffs). It asks the remote for its HEAD first, so an unchanged
// repository costs one small request; a changed one is cloned shallow (no tags, one branch) into pb_cache, read and
// deleted. Unchanged files aren't embedded again.
func (s *service) readGit(b, src *core.Record, force bool) (string, error) {
	sp := specOf(src)
	ctx, cancel := context.WithTimeout(context.Background(), GitTimeout)
	defer cancel()

	head, err := remoteHead(ctx, sp.URL)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", sp.URL, err)
	}
	if !force && head != "" && head == src.GetString("state") {
		return head, nil
	}

	commits := sp.Commits
	switch {
	case commits == 0:
		commits = DefaultCommits
	case commits < 0:
		commits = 0
	}
	commits = min(commits, MaxCommits)

	cache, err := extkit.CacheDir(s.app, Name)
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(cache, "git-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	repo, err := git.PlainCloneContext(ctx, filepath.Join(dir, "repo"), true, &git.CloneOptions{
		URL: sp.URL, Depth: max(commits, 1), SingleBranch: true, Tags: git.NoTags,
	})
	if err != nil {
		return "", fmt.Errorf("cloning %s: %w", sp.URL, err)
	}
	ref, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("%s: no default branch: %w", sp.URL, err)
	}
	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return "", err
	}
	tree, err := commit.Tree()
	if err != nil {
		return "", err
	}

	var docs []docIn
	err = tree.Files().ForEach(func(f *object.File) error {
		if len(docs) >= MaxRepoFiles {
			return io.EOF
		}
		if !f.Mode.IsFile() || f.Size > MaxMultiFileBytes || skipPath(f.Name, sp.Exclude) {
			return nil
		}
		if bin, err := f.IsBinary(); err != nil || bin {
			return nil
		}
		text, err := f.Contents()
		if err != nil || !isText([]byte(text)) {
			return nil
		}
		docs = append(docs, docIn{name: f.Name, text: text})
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	if commits > 0 {
		iter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
		if err == nil {
			n := 0
			_ = iter.ForEach(func(c *object.Commit) error {
				if n >= commits {
					return io.EOF
				}
				n++
				docs = append(docs, docIn{name: "commit " + c.Hash.String()[:10], text: commitText(ctx, c), kind: KindMarkdown})
				return nil
			})
		}
	}
	if err := s.syncDocs(b, src, docs, force); err != nil {
		return "", err
	}
	return ref.Hash().String(), nil
}

// remoteHead is the commit the remote's HEAD points at, asked without cloning.
func remoteHead(ctx context.Context, url string) (string, error) {
	r := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{Name: "origin", URLs: []string{url}})
	refs, err := r.ListContext(ctx, &git.ListOptions{})
	if err != nil {
		return "", err
	}
	byName := map[plumbing.ReferenceName]*plumbing.Reference{}
	for _, ref := range refs {
		byName[ref.Name()] = ref
	}
	head := byName[plumbing.HEAD]
	for i := 0; head != nil && head.Type() == plumbing.SymbolicReference && i < 5; i++ {
		head = byName[head.Target()]
	}
	if head == nil {
		return "", nil
	}
	return head.Hash().String(), nil
}

// commitText is a commit as a document: its subject as the title, then its body, author, date and the files it
// changed (as far as the shallow history reaches).
func commitText(ctx context.Context, c *object.Commit) string {
	subject, body, _ := strings.Cut(strings.TrimSpace(c.Message), "\n")
	var b strings.Builder
	b.WriteString("# " + strings.TrimSpace(subject) + "\n\n")
	if body = strings.TrimSpace(body); body != "" {
		b.WriteString(body + "\n\n")
	}
	fmt.Fprintf(&b, "Commit %s by %s on %s.", c.Hash.String()[:10], c.Author.Name, c.Author.When.UTC().Format("2006-01-02"))
	if files := changedFiles(ctx, c); len(files) > 0 {
		more := ""
		if len(files) > commitFilesShown {
			more = fmt.Sprintf(" and %d more", len(files)-commitFilesShown)
			files = files[:commitFilesShown]
		}
		b.WriteString("\n\nFiles: " + strings.Join(files, ", ") + more + ".")
	}
	return b.String()
}

// changedFiles are the paths a commit changed against its first parent (none past the shallow history's edge).
func changedFiles(ctx context.Context, c *object.Commit) []string {
	tree, err := c.Tree()
	if err != nil {
		return nil
	}
	var parentTree *object.Tree
	if c.NumParents() > 0 {
		p, err := c.Parent(0)
		if err != nil {
			return nil
		}
		if parentTree, err = p.Tree(); err != nil {
			return nil
		}
	}
	changes, err := object.DiffTreeWithOptions(ctx, parentTree, tree, nil)
	if err != nil {
		return nil
	}
	var out []string
	for _, ch := range changes {
		name := ch.To.Name
		if name == "" {
			name = ch.From.Name
		}
		out = append(out, name)
	}
	return out
}
