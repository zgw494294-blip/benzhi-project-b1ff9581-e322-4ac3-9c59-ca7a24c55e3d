package retest_idempotency_replay_test

import (
	"context"
	"testing"
	"time"

	"github.com/benzhi/ancient-tree-pathogen/internal/application"
	"github.com/benzhi/ancient-tree-pathogen/internal/storage"
)

func TestRetestIdempotencyKeyCanBeReplayed(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil { t.Fatal(err) }
	defer db.Close()
	app := application.New(db)
	ctx := context.Background()
	now := time.Now().UTC().Add(-time.Minute)
	c, err := app.CreateCase(ctx, "TREE-REPLAY-1", "银杏", "东区", "保护中心", "登记员")
	if err != nil { t.Fatal(err) }
	if _, err = app.AddSample(ctx, c.ID, "S-REPLAY", "采样员", "SEAL-REPLAY", "检测员", "完整", now, now, 1); err != nil { t.Fatal(err) }
	if _, err = app.AddTest(ctx, c.ID, "PCR", "检测员", "腐霉", "低", "qPCR", "阴性", "", now); err != nil { t.Fatal(err) }
	if _, err = app.Review(ctx, c.ID, "通过", "隔离病灶并复检", "复核员", 3); err != nil { t.Fatal(err) }
	first, err := app.RetestWithKey(ctx, c.ID, "通过", "复检员", "首次复检", "replay-key-1", 4)
	if err != nil { t.Fatal(err) }
	second, err := app.RetestWithKey(ctx, c.ID, "通过", "复检员", "首次复检", "replay-key-1", 4)
	if err != nil { t.Fatalf("same idempotent request was rejected on retry: %v", err) }
	if second.ID != first.ID || second.Status != first.Status { t.Fatalf("replay returned a different result: %#v vs %#v", second, first) }
}
