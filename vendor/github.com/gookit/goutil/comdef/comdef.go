// Package comdef provide some common type or constant definitions
package comdef

type (
	// MarshalFunc define
	MarshalFunc func(v any) ([]byte, error)

	// UnmarshalFunc define
	UnmarshalFunc func(bts []byte, ptr any) error
)

// IntCheckFunc check func
type IntCheckFunc func(val int) error

// StrCheckFunc check func
type StrCheckFunc func(val string) error

// ToStringFunc try to convert value to string, return error on fail
type ToStringFunc func(v any) (string, error)

// SafeStringFunc safe convert value to string
type SafeStringFunc func(v any) string
