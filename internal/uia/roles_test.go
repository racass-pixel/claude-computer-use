package uia

import "testing"

func TestRoles(t *testing.T) {
	if id, ok := RoleID("button"); !ok || id != 50000 {
		t.Fatalf("button → %d %v", id, ok)
	}
	if id, _ := RoleID("MenuItem"); id != 50011 {
		t.Fatalf("MenuItem → %d", id)
	}
	if RoleName(50004) != "Edit" || RoleName(99999) != "Custom" {
		t.Fatal("RoleName wrong")
	}
	if _, ok := RoleID("nope"); ok {
		t.Fatal("unknown role must not resolve")
	}
}
