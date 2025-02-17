package lambda

func NotNil[T any](t *T) bool {
	return t != nil
}

func Filter[T any](ts []T, fn func(T) bool) []T {
	var filtered []T
	for _, t := range ts {
		if !fn(t) {
			continue
		}
		filtered = append(filtered, t)
	}
	return filtered
}
