package helper

import (
	"testing"
)

func TestHelperAllowedAction(t *testing.T) {
	s := &Server{}
	req := Request{
		Action: "shares.apply",
		Plan:   true,
		Args: map[string]interface{}{
			"name": "documents",
			"path": "/data/documents",
		},
	}

	res := s.processRequest(req)
	if !res.Ok {
		t.Fatalf("expected action to succeed, got error: %s", res.Error)
	}
	if len(res.Plan) != 3 {
		t.Errorf("expected 3 plan steps, got %d", len(res.Plan))
	}
}

func TestHelperDisallowedAction(t *testing.T) {
	s := &Server{}
	req := Request{
		Action: "dangerous.rmrf",
		Plan:   false,
		Args:   map[string]interface{}{},
	}

	res := s.processRequest(req)
	if res.Ok {
		t.Errorf("expected disallowed action to be rejected")
	}
}

func TestHelperPathTraversalRejected(t *testing.T) {
	s := &Server{}
	req := Request{
		Action: "shares.apply",
		Plan:   true,
		Args: map[string]interface{}{
			"name": "attack",
			"path": "/data/../../etc/shadow",
		},
	}

	res := s.processRequest(req)
	if res.Ok {
		t.Errorf("expected path traversal to be rejected")
	}
}
