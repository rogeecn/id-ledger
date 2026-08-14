package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("ID_LEDGER_TOKEN", "secret")
	t.Setenv("ID_LEDGER_ADDR", ":4000")
	t.Setenv("ID_LEDGER_DATABASE_PATH", "test.db")

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Token != "secret" || got.Addr != ":4000" || got.DatabasePath != "test.db" {
		t.Fatalf("unexpected config: %#v", got)
	}
}

func TestLoadRequiresToken(t *testing.T) {
	t.Setenv("ID_LEDGER_TOKEN", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected missing token error")
	}
}
