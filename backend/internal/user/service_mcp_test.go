package user

import (
	"context"
	"testing"

	_ "github.com/lib-x/entsqlite"
	"owl/backend/ent"
)

func TestMCPTokenGenerationAndAuthentication(t *testing.T) {
	ctx := context.Background()
	client, err := ent.Open("sqlite3", "file:user_mcp_token?mode=memory&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Schema.Create(ctx); err != nil {
		t.Fatal(err)
	}

	svc := NewService(client, "secret", t.TempDir())
	u, err := client.User.Create().SetUsername("alice").SetDisplayName("Alice").SetPasswordHash("hash").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	generated, err := svc.GenerateMCPToken(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !generated.Configured || generated.Token == "" || generated.Hint == "" {
		t.Fatalf("unexpected generated token status: %#v", generated)
	}

	status, err := svc.GetMCPTokenStatus(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Configured || status.Token != "" || status.Hint != generated.Hint {
		t.Fatalf("stored status should only expose hint: %#v", status)
	}

	claims, err := svc.AuthenticateMCPToken(ctx, generated.Token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != u.ID || claims.Username != "alice" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
	if _, err := svc.AuthenticateMCPToken(ctx, generated.Token+"bad"); err == nil {
		t.Fatal("expected invalid token to be rejected")
	}

	bob, err := client.User.Create().SetUsername("bob").SetDisplayName("Bob").SetPasswordHash("hash").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetMCPToken(ctx, bob.ID, generated.Token); err == nil {
		t.Fatal("expected duplicate token to be rejected")
	}

	deleted, err := svc.DeleteMCPToken(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if deleted.Configured || deleted.Hint != "" || deleted.Token != "" {
		t.Fatalf("unexpected deleted token status: %#v", deleted)
	}
	if _, err := svc.AuthenticateMCPToken(ctx, generated.Token); err == nil {
		t.Fatal("expected deleted token to be rejected")
	}
}
