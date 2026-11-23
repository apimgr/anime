package database

import (
	"context"
	"testing"

	"github.com/apimgr/anime/src/config"
)

func TestAppSecretsArePersistentAndNotReplaced(t *testing.T) {
	db, err := Open(config.DatabaseConfig{Driver: "sqlite", URL: ":memory:"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.SQL.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	}()

	first, err := AppSecret(context.Background(), db.SQL, "installation_secret")
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 32 {
		t.Fatalf("installation secret length = %d, want 32", len(first))
	}
	if err := EnsureAppSecrets(context.Background(), db.SQL); err != nil {
		t.Fatal(err)
	}
	second, err := AppSecret(context.Background(), db.SQL, "installation_secret")
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("EnsureAppSecrets replaced the persisted installation secret")
	}
}
