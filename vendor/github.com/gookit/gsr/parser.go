package gsr

// Marshaler interface
type Marshaler interface {
	Marshal(v any) ([]byte, error)
}

// Unmarshaler interface
type Unmarshaler interface {
	Unmarshal(v []byte, ptr any) error
}

// DataParser interface for Marshal/Unmarshal data
type DataParser interface {
	Marshaler
	Unmarshaler
}

// MarshalFunc define
type MarshalFunc func(v any) ([]byte, error)

// Marshal implements the Marshaler
func (m MarshalFunc) Marshal(v any) ([]byte, error) {
	return m(v)
}

// UnmarshalFunc define
type UnmarshalFunc func(v []byte, ptr any) error

// Unmarshal implements the Unmarshaler
func (u UnmarshalFunc) Unmarshal(v []byte, ptr any) error {
	return u(v, ptr)
}
