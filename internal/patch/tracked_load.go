package patch

import "context"

func (r repository) loadTracked(
	ctx context.Context,
	oldRevision string,
) ([]File, error) {
	raw, err := r.gitDiff(ctx, oldRevision)
	if err != nil {
		return nil, err
	}
	files, err := parseTracked(ctx, raw)
	if err != nil {
		return nil, err
	}
	for index := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file := &files[index]
		file.OldSource = r.readGitSource(ctx, oldRevision, file.OldPath)
		file.NewSource = r.readWorkingTree(file.NewPath)
	}
	return files, nil
}
