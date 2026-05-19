package views

const (
	L0 = "L0"
	L1 = "L1"
	L2 = "L2"
	L3 = "L3"
)

// IsValidLevel reports whether s is a recognized detail level constant.
func IsValidLevel(s string) bool {
	return s == L0 || s == L1 || s == L2 || s == L3
}
