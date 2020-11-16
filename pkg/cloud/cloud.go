// Package cloud provides cloud generic operations.
package cloud

type Cloud interface {
	Login() (map[string]string, error)
}
