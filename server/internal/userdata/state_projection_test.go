package userdata

import (
	"strings"
	"testing"
	"time"

	"lunar-tear/server/internal/store"
)

func TestFirstEntranceSnapshotAndDiffFormatting(t *testing.T) {
	userStore := store.New(func() time.Time {
		return time.Unix(1_700_000_000, 0)
	})
	user := userStore.EnsureUser("user-1")

	firstEntrance := FirstEntranceClientTableMap(user)
	if got := firstEntrance["IUserCharacter"]; got != "[]" {
		t.Fatalf("IUserCharacter = %q, want []", got)
	}
	if got := firstEntrance["IUserMainQuestFlowStatus"]; got != "[]" {
		t.Fatalf("IUserMainQuestFlowStatus = %q, want []", got)
	}
	if !strings.Contains(firstEntrance["IUserProfile"], `"name":"Un-regist User Name"`) {
		t.Fatalf("IUserProfile should keep first-entrance name, got %s", firstEntrance["IUserProfile"])
	}

	full := FullClientTableMap(user)
	if got := full["IUserCharacter"]; got == "[]" {
		t.Fatal("IUserCharacter should be populated in full snapshot")
	}
	if got := full["IUserQuest"]; got == "[]" {
		t.Fatal("IUserQuest should be populated in full snapshot")
	}

	selected := SelectTables(full, []string{"IUserProfile", "IUserGimmick", "MissingTable"})
	diff := BuildDiffFromTables(selected)
	if diff["IUserProfile"].DeleteKeysJson != "[]" {
		t.Fatalf("DeleteKeysJson = %q, want []", diff["IUserProfile"].DeleteKeysJson)
	}
	if diff["MissingTable"].UpdateRecordsJson != "[]" {
		t.Fatalf("missing table payload = %q, want []", diff["MissingTable"].UpdateRecordsJson)
	}
}
