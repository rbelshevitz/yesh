//go:build windows

package main

import "fmt"

func (r *runtime) storageQuery(name string, args ...string) ([]byte, error) {
	return r.storageQueryPolicy(false, name, args...)
}
func (r *runtime) storageQueryPolicy(strict bool, name string, args ...string) ([]byte, error) {
	return nil, fmt.Errorf("storage diagnostics require Linux and util-linux")
}
