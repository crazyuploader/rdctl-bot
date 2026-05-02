package db

import "testing"

func TestClose_NilPool(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Close(nil) panicked: %v", r)
		}
	}()
	Close(nil)
}
