package utils

func Clone[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
