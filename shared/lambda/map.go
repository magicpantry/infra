package lambda

func Dereference[T any](t *T) T {
	return *t
}

func Map[X, Y any](xs []X, fn func(X) Y) []Y {
	var ys []Y
	for _, x := range xs {
		ys = append(ys, fn(x))
	}
	return ys
}
