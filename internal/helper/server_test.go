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

func TestHelperSharesBindPhotosPlan(t *testing.T) {
	s := &Server{}
	// Global bind
	req := Request{
		Action: "shares.bind_photos",
		Plan:   true,
		Args: map[string]interface{}{
			"enabled": true,
		},
	}
	res := s.processRequest(req)
	if !res.Ok {
		t.Fatalf("expected shares.bind_photos to succeed, got: %s", res.Error)
	}
	if len(res.Plan) == 0 {
		t.Errorf("expected at least 1 plan item for bind_photos, got 0")
	}

	// Per-user bind
	reqUser := Request{
		Action: "shares.bind_photos",
		Plan:   true,
		Args: map[string]interface{}{
			"enabled":  true,
			"username": "mario",
		},
	}
	resUser := s.processRequest(reqUser)
	if !resUser.Ok {
		t.Fatalf("expected per-user bind_photos to succeed, got: %s", resUser.Error)
	}
	if len(resUser.Plan) != 1 {
		t.Errorf("expected exactly 1 plan item for user mario, got %d", len(resUser.Plan))
	}
}

func TestHelperSharesSetPasswordPlan(t *testing.T) {
	s := &Server{}
	req := Request{
		Action: "shares.set_password",
		Plan:   true,
		Args: map[string]interface{}{
			"username": "mario",
			"password": "supersecretpassword",
		},
	}
	res := s.processRequest(req)
	if !res.Ok {
		t.Fatalf("expected shares.set_password to succeed, got: %s", res.Error)
	}
	if len(res.Plan) == 0 {
		t.Errorf("expected plan items, got 0")
	}
}

func TestHelperUsersCreatePlan(t *testing.T) {
	s := &Server{}
	req := Request{
		Action: "users.create",
		Plan:   true,
		Args: map[string]interface{}{
			"username": "mario",
		},
	}
	res := s.processRequest(req)
	if !res.Ok {
		t.Fatalf("expected users.create to succeed, got: %s", res.Error)
	}
	if len(res.Plan) == 0 {
		t.Errorf("expected plan items for users.create, got 0")
	}
}
