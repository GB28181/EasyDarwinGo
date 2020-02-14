package utils

// Command wrapper
type Command interface {
	Do() error
}
