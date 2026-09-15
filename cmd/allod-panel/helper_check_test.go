package main

import (
	"testing"
)

func TestCheckHelperConnectivity_Offline(t *testing.T) {
	connected, _ := checkHelperConnectivity()
	if connected {
		t.Errorf("expected connected=false when helper is not running in test, got true")
	}
}
