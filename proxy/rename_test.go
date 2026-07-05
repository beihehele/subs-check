package proxies

import "testing"

func TestFormatRename_WithSeq(t *testing.T) {
	got := FormatRename("HK", 1)
	if got != "🇭🇰HK_1" {
		t.Fatalf("FormatRename(HK,1) = %q, want 🇭🇰HK_1", got)
	}
	got = FormatRename("HK", 2)
	if got != "🇭🇰HK_2" {
		t.Fatalf("FormatRename(HK,2) = %q, want 🇭🇰HK_2", got)
	}
}

func TestFormatRename_PreviewNoSeq(t *testing.T) {
	got := FormatRename("HK", 0)
	if got != "🇭🇰HK" {
		t.Fatalf("FormatRename(HK,0) = %q, want 🇭🇰HK", got)
	}
}

func TestFormatRename_UnknownCountry(t *testing.T) {
	got := FormatRename("", 1)
	if got != "❓Other_1" {
		t.Fatalf("FormatRename(empty,1) = %q, want ❓Other_1", got)
	}
	got = FormatRename("", 0)
	if got != "❓Other" {
		t.Fatalf("FormatRename(empty,0) = %q, want ❓Other", got)
	}
}

func TestRenameSeqKey(t *testing.T) {
	if RenameSeqKey("hk") != "HK" {
		t.Fatalf("expected HK bucket")
	}
	if RenameSeqKey("") != "" {
		t.Fatalf("empty country should share Other bucket")
	}
	if RenameSeqKey("NOTACOUNTRY") != "" {
		t.Fatalf("unknown country should share Other bucket")
	}
}
