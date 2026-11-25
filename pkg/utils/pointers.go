package utils

// Deref returns the value pointed to by ptr, or the zero value of T if ptr is nil.
func Deref[T any](ptr *T) T {
	var zero T
	if ptr == nil {
		return zero
	}

	return *ptr
}
