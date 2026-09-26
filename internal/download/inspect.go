package download

import "context"

func (e *Engine) InspectSize(ctx context.Context, url string) (int64, error) {
	m, err := e.probe(ctx, url)
	return m.size, err
}
