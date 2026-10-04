package git

import (
	"context"
	"errors"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type Repository struct {
	root   string
	loader Loader
}

func Open(ctx context.Context, directory string) (*Repository, error) {
	loader := Loader{Runner: ExecRunner{}}
	root, err := loader.Root(ctx, directory)
	if err != nil {
		return nil, err
	}
	return &Repository{root: root, loader: loader}, nil
}

func (r *Repository) Root() string {
	if r == nil {
		return ""
	}
	return r.root
}

func (r *Repository) Load(ctx context.Context, base string) (patch.Patch, error) {
	if r == nil || r.root == "" {
		return patch.Patch{}, errors.New("repository is not open")
	}
	if base == "" {
		return r.loader.Load(ctx, r.root)
	}
	return r.loader.LoadBranch(ctx, r.root, base)
}

func (r *Repository) DefaultBranch(ctx context.Context) (string, error) {
	if r == nil || r.root == "" {
		return "", errors.New("repository is not open")
	}
	return r.loader.DefaultBranch(ctx, r.root)
}
