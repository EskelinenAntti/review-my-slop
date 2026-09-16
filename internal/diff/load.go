package diff

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

func (l Loader) Load(ctx context.Context, dir string) (ChangeSet, error) {
	root, err := l.Root(ctx, dir)
	if err != nil {
		return ChangeSet{}, err
	}
	raw, err := l.loadDiff(ctx, root)
	if err != nil {
		return ChangeSet{}, err
	}
	return l.assemble(ctx, root, "", raw, readIndex)
}

func (l Loader) LoadBranch(ctx context.Context, dir, branch string) (ChangeSet, error) {
	root, err := l.Root(ctx, dir)
	if err != nil {
		return ChangeSet{}, err
	}
	baseBytes, err := l.gitRunner().Run(ctx, root, "merge-base", branch, "HEAD")
	if err != nil {
		return ChangeSet{}, fmt.Errorf("find branch point with %s: %w", branch, err)
	}
	base := strings.TrimSpace(string(baseBytes))
	raw, err := l.loadDiff(ctx, root, base)
	if err != nil {
		return ChangeSet{}, err
	}
	readBase := func(ctx context.Context, runner Runner, repository, path string) string {
		return readRevision(ctx, runner, repository, base, path)
	}
	return l.assemble(ctx, root, branch, raw, readBase)
}

func (l Loader) DefaultBranch(ctx context.Context, dir string) (string, error) {
	root, err := l.Root(ctx, dir)
	if err != nil {
		return "", err
	}
	return l.defaultBranch(ctx, root), nil
}

type sourceReader func(context.Context, Runner, string, string) string

func (l Loader) assemble(ctx context.Context, root, base string, raw []byte, readOld sourceReader) (ChangeSet, error) {
	files, err := parseTracked(ctx, l.gitRunner(), root, raw, readOld)
	if err != nil {
		return ChangeSet{}, err
	}
	untracked, err := l.loadUntracked(ctx, root)
	if err != nil {
		return ChangeSet{}, err
	}
	files = append(files, untracked...)
	sort.SliceStable(files, func(i, j int) bool { return files[i].DisplayPath < files[j].DisplayPath })

	hash := sha256.New()
	_, _ = hash.Write([]byte(base))
	_, _ = hash.Write(raw)
	for _, file := range untracked {
		_, _ = hash.Write([]byte(file.NewPath))
		_, _ = hash.Write([]byte(file.NewSource))
	}

	return ChangeSet{
		Repository:  root,
		Fingerprint: hex.EncodeToString(hash.Sum(nil)),
		Files:       files,
	}, nil
}
