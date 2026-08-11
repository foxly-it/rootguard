package updater

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCleanupPreviewKeepsTwoImagesPerServiceAndEstimatesEligibleResources(t *testing.T) {
	manager := NewManager(Options{
		DataDir: t.TempDir(),
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			command := strings.Join(arguments, " ")
			switch command {
			case "ps -a --filter ancestor=unbound-old --format {{.ID}}", "ps -a --filter ancestor=adguard-old --format {{.ID}}", "ps -a --filter volume=transient --format {{.ID}}":
				return nil, nil
			case "system df -v --format {{json .Images}}":
				return []byte(`[{"ID":"unbound-old","UniqueSize":"12MB"},{"ID":"adguard-old","UniqueSize":"8MB"}]`), nil
			case "volume ls --quiet --filter label=io.rootguard.cleanup=true":
				return []byte("transient\nin-use\n"), nil
			case "ps -a --filter volume=in-use --format {{.ID}}":
				return []byte("container"), nil
			case "system df -v --format {{json .Volumes}}":
				return []byte(`[{"Name":"transient","Size":"1.5MB"},{"Name":"in-use","Size":"2MB"}]`), nil
			default:
				return nil, errors.New("unexpected command: " + command)
			}
		},
	})
	manager.status.History = []HistoryEntry{
		{Service: "unbound", Outcome: "success", FromID: "unbound-previous", ToID: "unbound-current"},
		{Service: "adguard", Outcome: "success", FromID: "adguard-previous", ToID: "adguard-current"},
		{Service: "unbound", Outcome: "success", FromID: "unbound-old", ToID: "unbound-previous"},
		{Service: "adguard", Outcome: "success", FromID: "adguard-old", ToID: "adguard-previous"},
	}

	preview, err := manager.PreviewCleanup(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Resources) != 3 || preview.EstimatedBytes != 21_500_000 {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if strings.Join(preview.Skipped, ",") != "volume:in-use" {
		t.Fatalf("expected used volume to be skipped: %+v", preview)
	}
	for _, protected := range []string{"unbound-current", "unbound-previous", "adguard-current", "adguard-previous"} {
		for _, resource := range preview.Resources {
			if resource.ID == protected {
				t.Fatalf("protected image %q became eligible", protected)
			}
		}
	}
}

func TestRunManualCleanupRecomputesAndRecordsResult(t *testing.T) {
	removeCalls := 0
	manager := NewManager(Options{
		DataDir: t.TempDir(),
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			command := strings.Join(arguments, " ")
			switch command {
			case "ps -a --filter ancestor=old --format {{.ID}}":
				return nil, nil
			case "system df -v --format {{json .Images}}":
				return []byte(`[{"ID":"old","UniqueSize":"42B"}]`), nil
			case "volume ls --quiet --filter label=io.rootguard.cleanup=true":
				return nil, nil
			case "image rm old":
				removeCalls++
				return []byte("removed"), nil
			default:
				return nil, errors.New("unexpected command: " + command)
			}
		},
	})
	manager.status.History = []HistoryEntry{
		{Service: "unbound", Outcome: "success", FromID: "previous", ToID: "current"},
		{Service: "unbound", Outcome: "success", FromID: "old", ToID: "previous"},
	}

	result, err := manager.RunManualCleanup(context.Background())
	if err != nil || removeCalls != 1 || strings.Join(result.RemovedImages, ",") != "old" {
		t.Fatalf("unexpected cleanup: result=%+v calls=%d err=%v", result, removeCalls, err)
	}
	status := manager.Status()
	if status.State != StateIdle || len(status.History) != 3 || status.History[0].Outcome != "cleanup" || strings.Join(status.History[0].Cleanup.RemovedImages, ",") != "old" {
		t.Fatalf("manual cleanup was not recorded: %+v", status)
	}
}

func TestCleanupPreviewRejectsBusyManagerAndParsesDockerSizes(t *testing.T) {
	manager := NewManager(Options{DataDir: t.TempDir()})
	manager.status.State = StateUpdating
	if _, err := manager.PreviewCleanup(context.Background()); !errors.Is(err, ErrBusy) {
		t.Fatalf("expected busy error, got %v", err)
	}
	for value, expected := range map[string]int64{"0B": 0, "1.5kB": 1500, "2MB": 2_000_000, "3GB": 3_000_000_000} {
		actual, ok := parseDockerSize(value)
		if !ok || actual != expected {
			t.Fatalf("parseDockerSize(%q) = %d, %v", value, actual, ok)
		}
	}
	if _, ok := parseDockerSize("unknown"); ok {
		t.Fatal("expected unknown Docker size to be rejected")
	}
}
