package issueopsinventory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"issueops/internal/adapter/outbound/issueopsrecord"
	"issueops/internal/adapter/outbound/sqlstore"
	inventoryapp "issueops/internal/application/issueopsinventory"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type inventoryNormalizer struct{ calls int }

func (paths *inventoryNormalizer) Normalize(path string) string {
	paths.calls++
	return path
}

type inventoryClock struct{}

func (inventoryClock) Now() time.Time { return time.Unix(0, 0).UTC() }

func seedFilteredInventory(tb testing.TB, count int) string {
	tb.Helper()
	root := tb.TempDir()
	database, err := sqlstore.Open(root)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		if err := sqlstore.CloseRoot(root); err != nil {
			tb.Error(err)
		}
	})
	mutations := make([]port.RecordMutation, 0, count)
	for index := range count {
		repo := "/foreign"
		if index%10 == 0 {
			repo = "/repo"
		}
		id := fmt.Sprintf("io-%05d", index)
		data, err := issueopsrecord.Encode(issueopscontract.IssueOpsRecord{
			SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
			ID:            id, Repo: repo, Phase: issueopscontract.IssueOpsPhaseProblem,
		})
		if err != nil {
			tb.Fatal(err)
		}
		mutations = append(mutations, port.RecordMutation{
			Bucket: issueopsrecord.Bucket(), ID: id, Data: data,
		})
	}
	if err := database.Apply(context.Background(), mutations); err != nil {
		tb.Fatal(err)
	}
	return root
}

func TestFilteredPersistedInventoryReusesRequestedNormalization(t *testing.T) {
	root := seedFilteredInventory(t, 100)
	paths := &inventoryNormalizer{}
	service := inventoryapp.NewService(Repository{}, inventoryClock{}, paths)
	result, err := service.ListCycles(context.Background(), root, "/repo")
	if err != nil {
		t.Fatal(err)
	}
	if result.ScannedRecords != 100 || len(result.Entries) != 10 {
		t.Fatalf("unexpected inventory: %+v", result)
	}
	if paths.calls != 2 {
		t.Fatalf("normalizations = %d, want 2 distinct paths including requested repo", paths.calls)
	}
}

func TestFilteredPersistedInventoryRetainsForeignDiagnosticsAndSorting(t *testing.T) {
	root := seedFilteredInventory(t, 100)
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"io-00002", "io-00001"} {
		data := fmt.Appendf(nil, `{"schema_version":1,"id":%q,"repo":"/foreign","phase":"problem","unknown":true}`, id)
		if err := db.Put(issueopsrecord.Bucket(), id, data); err != nil {
			t.Fatal(err)
		}
	}
	service := inventoryapp.NewService(Repository{}, inventoryClock{}, &inventoryNormalizer{})
	result, err := service.ListCycles(context.Background(), root, "/repo")
	if err != nil {
		t.Fatal(err)
	}
	if result.ScannedRecords != 100 || result.ReadErrors != 2 || len(result.Entries) != 10 ||
		!slices.Equal(result.UnreadableIDs, []string{"io-00001", "io-00002"}) {
		t.Fatalf("inventory lost rows or diagnostics: %+v", result)
	}
	if len(result.Diagnostics) != 2 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
	for index, diagnostic := range result.Diagnostics {
		if diagnostic.ID != result.UnreadableIDs[index] || diagnostic.Code != "invalid_state" {
			t.Fatalf("diagnostic = %+v", diagnostic)
		}
	}
	for index, entry := range result.Entries {
		if entry.ID != fmt.Sprintf("io-%05d", index*10) || entry.Repo != "/repo" {
			t.Fatalf("entry %d = %+v", index, entry)
		}
	}
}

func TestStreamingInventoryMissingStoreIsReadOnly(t *testing.T) {
	parent := t.TempDir()
	service := inventoryapp.NewService(Repository{}, inventoryClock{}, &inventoryNormalizer{})
	result, err := service.ListCycles(context.Background(), filepath.Join(parent, "missing"), "/repo")
	if err != nil || !result.OK || result.ScannedRecords != 0 ||
		result.Entries == nil || result.Diagnostics == nil || result.UnreadableIDs == nil {
		t.Fatalf("missing inventory = %+v, %v", result, err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("missing read created state: %v, %v", entries, err)
	}
}

func TestStreamingInventoryHonorsCancellationDuringScan(t *testing.T) {
	root := seedFilteredInventory(t, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	visited := 0
	diagnostics, err := (Repository{}).ScanEach(ctx, root, func(_ issueopscontract.IssueOpsRecord) error {
		visited++
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) || visited != 1 || diagnostics != nil {
		t.Fatalf("cancelled scan: visited=%d diagnostics=%v err=%v", visited, diagnostics, err)
	}
}

func BenchmarkFilteredPersistedInventory(b *testing.B) {
	for _, count := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("rows_%d", count), func(b *testing.B) {
			root := seedFilteredInventory(b, count)
			paths := &inventoryNormalizer{}
			service := inventoryapp.NewService(Repository{}, inventoryClock{}, paths)
			b.ReportAllocs()
			for b.Loop() {
				result, err := service.ListCycles(context.Background(), root, "/repo")
				if err != nil {
					b.Fatal(err)
				}
				if result.ScannedRecords != count || len(result.Entries) != count/10 {
					b.Fatalf("scanned=%d entries=%d", result.ScannedRecords, len(result.Entries))
				}
			}
			b.ReportMetric(float64(paths.calls)/float64(b.N), "normalizations/op")
		})
	}
}
