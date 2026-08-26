package verification_receipt_cache_alias_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/benzhi/ancient-tree-pathogen/internal/application"
	"github.com/benzhi/ancient-tree-pathogen/internal/storage"
)

func TestVerificationReceiptCacheIsolatedFromCallerMutation(t *testing.T) {
	ctx := context.Background()
	repo, err := storage.Open(filepath.Join(t.TempDir(), "receipt.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	stored, err := repo.VerifyCredentialBatch(ctx, []string{"not-a-credential"})
	if err != nil {
		t.Fatal(err)
	}
	service := application.New(repo)

	first, err := service.VerificationReceipt(ctx, stored.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 1 {
		t.Fatalf("expected one persisted verification item, got %d", len(first.Items))
	}
	want := first.Items[0].Conclusion
	first.Items[0].Conclusion = "caller-mutated"

	again, err := service.VerificationReceipt(ctx, stored.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Items[0].Conclusion; got != want {
		t.Fatalf("cached receipt leaked caller mutation: got %q, want persisted %q", got, want)
	}
}
