// Package clocks contains the production clock adapter.
package clocks

import "time"

type System struct{}

func NewSystem() System       { return System{} }
func (System) Now() time.Time { return time.Now().UTC() }
