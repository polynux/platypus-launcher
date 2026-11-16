// decodeRunningApps unit tests: the NotifyRunningApps shapes the helper
// parses, built with depot.Enc (no CM, no network). Field numbers per the
// live push (issue #178): 1 the family group, repeated 2 the entries
// {1: appid, 3: {1: player, 2: owner}}, field 3 repeating when several
// members play one app.
package main

import (
	"testing"

	"platypus/internal/depot"
)

func runningPair(player, owner uint64) []byte {
	p := &depot.Enc{}
	p.Fixed64(1, player)
	p.Fixed64(2, owner)
	return p.Bytes()
}

func runningEntryBytes(appID uint32, pairs ...[]byte) []byte {
	e := &depot.Enc{}
	e.Varint(1, uint64(appID))
	for _, p := range pairs {
		e.BytesField(3, p)
	}
	return e.Bytes()
}

func runningBodyBytes(entries ...[]byte) []byte {
	b := &depot.Enc{}
	b.Varint(1, 424242)
	for _, e := range entries {
		b.BytesField(2, e)
	}
	return b.Bytes()
}

func TestDecodeRunningAppsPairs(t *testing.T) {
	body := runningBodyBytes(
		runningEntryBytes(5002, runningPair(76561200, 76561199), runningPair(76561199, 76561200)),
		runningEntryBytes(5001, runningPair(76561200, 76561199)),
	)
	got := decodeRunningApps(body)
	want := []runningEntry{
		{appID: 5002, player: 76561200, owner: 76561199},
		{appID: 5002, player: 76561199, owner: 76561200},
		{appID: 5001, player: 76561200, owner: 76561199},
	}
	if len(got) != len(want) {
		t.Fatalf("decoded = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("decoded[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestDecodeRunningAppsSkipsMalformed(t *testing.T) {
	// A field-2 entry that isn't a message, an entry without an appid and
	// a pair without an owner are skipped; the good pair still decodes.
	badEntry := &depot.Enc{}
	badEntry.Varint(1, 0)
	badPair := &depot.Enc{}
	badPair.Fixed64(1, 76561200)
	body := runningBodyBytes(
		[]byte{0xff, 0xff},
		badEntry.Bytes(),
		runningEntryBytes(5003, badPair.Bytes()),
		runningEntryBytes(5001, runningPair(76561200, 76561199)),
	)
	got := decodeRunningApps(body)
	if len(got) != 1 || got[0] != (runningEntry{appID: 5001, player: 76561200, owner: 76561199}) {
		t.Fatalf("decoded = %+v, want the one good pair", got)
	}
}

func TestDecodeRunningAppsEmpty(t *testing.T) {
	if got := decodeRunningApps(nil); len(got) != 0 {
		t.Fatalf("empty body decoded = %+v, want none", got)
	}
	if got := decodeRunningApps((&depot.Enc{}).Bytes()); len(got) != 0 {
		t.Fatalf("no entries decoded = %+v, want none", got)
	}
}
