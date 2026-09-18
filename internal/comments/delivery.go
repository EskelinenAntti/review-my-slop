package comments

import "io"

func (s Store) Deliver(repository string, output io.Writer) error {
	snapshot, err := s.Snapshot(repository)
	if err != nil {
		return err
	}
	if err := WritePrompt(output, snapshot.Comments()); err != nil {
		return err
	}
	return s.Acknowledge(snapshot)
}
