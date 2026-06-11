package pgadmin

import "testing"

func TestBuildBucardoRoleSQL(t *testing.T) {
	got := buildBucardoRoleSQL(`sysi5adm`)
	want := `ALTER ROLE "sysi5adm" WITH SUPERUSER CREATEROLE REPLICATION;`
	if got != want {
		t.Fatalf("unexpected SQL:\nwant: %s\n got: %s", want, got)
	}
}

func TestBuildBucardoRoleSQLQuotesIdentifier(t *testing.T) {
	got := buildBucardoRoleSQL(`sync"user`)
	want := `ALTER ROLE "sync""user" WITH SUPERUSER CREATEROLE REPLICATION;`
	if got != want {
		t.Fatalf("unexpected quoted SQL:\nwant: %s\n got: %s", want, got)
	}
}
