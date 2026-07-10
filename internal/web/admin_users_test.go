package web

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jhash/tabitha/internal/db"
)

func renderAdminUsersTable(t *testing.T, users []db.User) string {
	t.Helper()
	var buf bytes.Buffer
	if err := adminUsersTable(users).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	return buf.String()
}

func TestAdminUsersTableListsEmailAndRole(t *testing.T) {
	users := []db.User{
		{ID: 1, Email: "jhash147@gmail.com", Name: "Jake", Role: db.UserRoleSuperadmin},
		{ID: 2, Email: "jeff@example.com", Name: "Jeff", Role: db.UserRoleUser},
	}
	html := renderAdminUsersTable(t, users)

	for _, want := range []string{"jhash147@gmail.com", "Jake", "superadmin", "jeff@example.com", "Jeff", "user"} {
		if !strings.Contains(html, want) {
			t.Errorf("expected table to contain %q, got: %s", want, html)
		}
	}
}

func TestAdminUsersTableShowsPromoteFormOnlyForNonSuperadmins(t *testing.T) {
	users := []db.User{
		{ID: 1, Email: "jhash147@gmail.com", Role: db.UserRoleSuperadmin},
		{ID: 2, Email: "jeff@example.com", Role: db.UserRoleUser},
	}
	html := renderAdminUsersTable(t, users)

	if strings.Contains(html, "/admin/users/1/promote") {
		t.Error("expected no promote form for a user who is already superadmin")
	}
	if !strings.Contains(html, "/admin/users/2/promote") {
		t.Errorf("expected a promote form pointing at /admin/users/2/promote, got: %s", html)
	}
}
