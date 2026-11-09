package scene

// Ptr sets an optional scalar field, including an explicit zero or false value.
// For numeric fields, specify the scalar type, for example Ptr[float32](15).
func Ptr[T any](value T) *T { return &value }
