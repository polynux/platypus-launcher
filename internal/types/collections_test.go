package types

import (
	"encoding/json"
	"testing"
)

func TestSmartFilterMigratesSavedDeckVerdict(t *testing.T) {
	var filter SmartFilter
	if err := json.Unmarshal([]byte(`{"deck":"verified"}`), &filter); err != nil {
		t.Fatal(err)
	}
	if filter.BadgeKind != "verdict" || filter.BadgeValue != "verified" {
		t.Fatalf("migrated filter = %+v", filter)
	}
	body, err := json.Marshal(filter)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"badge_kind":"verdict","badge_value":"verified"}` {
		t.Fatalf("saved migrated filter = %s", body)
	}
}
