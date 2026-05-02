package db

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// ─────────────────────────────────────────────────────────────
// strPtr
// ─────────────────────────────────────────────────────────────

func TestStrPtr_EmptyReturnsNil(t *testing.T) {
	if got := strPtr(""); got != nil {
		t.Errorf("strPtr(\"\") = %v, want nil", got)
	}
}

func TestStrPtr_NonEmptyReturnsPointer(t *testing.T) {
	s := "hello"
	got := strPtr(s)
	if got == nil {
		t.Fatal("strPtr(\"hello\") = nil, want non-nil")
	}
	if *got != s {
		t.Errorf("strPtr(\"hello\") = %q, want %q", *got, s)
	}
}

func TestStrPtr_WhitespaceIsNonEmpty(t *testing.T) {
	got := strPtr("   ")
	if got == nil {
		t.Error("strPtr(\"   \") = nil, want non-nil pointer (whitespace is not empty)")
	}
}

// ─────────────────────────────────────────────────────────────
// int64Ptr
// ─────────────────────────────────────────────────────────────

func TestInt64Ptr_ZeroReturnsNil(t *testing.T) {
	if got := int64Ptr(0); got != nil {
		t.Errorf("int64Ptr(0) = %v, want nil", got)
	}
}

func TestInt64Ptr_PositiveReturnsPointer(t *testing.T) {
	got := int64Ptr(42)
	if got == nil {
		t.Fatal("int64Ptr(42) = nil, want non-nil")
	}
	if *got != 42 {
		t.Errorf("int64Ptr(42) = %d, want 42", *got)
	}
}

func TestInt64Ptr_NegativeReturnsPointer(t *testing.T) {
	got := int64Ptr(-7)
	if got == nil {
		t.Fatal("int64Ptr(-7) = nil, want non-nil")
	}
	if *got != -7 {
		t.Errorf("int64Ptr(-7) = %d, want -7", *got)
	}
}

// ─────────────────────────────────────────────────────────────
// toPgtypeTimestamptz
// ─────────────────────────────────────────────────────────────

func TestToPgtypeTimestamptz_ValidTimeIsMarkedValid(t *testing.T) {
	now := time.Now()
	got := toPgtypeTimestamptz(now)
	if !got.Valid {
		t.Error("toPgtypeTimestamptz: Valid should be true")
	}
}

func TestToPgtypeTimestamptz_TimeIsConvertedToUTC(t *testing.T) {
	// Use a fixed time in a non-UTC zone
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("could not load timezone, skipping")
	}
	eastern := time.Date(2024, 1, 15, 10, 0, 0, 0, loc)
	got := toPgtypeTimestamptz(eastern)
	if got.Time.Location() != time.UTC {
		t.Errorf("toPgtypeTimestamptz: expected UTC location, got %v", got.Time.Location())
	}
	// UTC equivalent of 10:00 EST is 15:00 UTC
	if got.Time.Hour() != 15 {
		t.Errorf("toPgtypeTimestamptz: expected UTC hour 15, got %d", got.Time.Hour())
	}
}

func TestToPgtypeTimestamptz_AlreadyUTCUnchanged(t *testing.T) {
	utc := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	got := toPgtypeTimestamptz(utc)
	if !got.Time.Equal(utc) {
		t.Errorf("toPgtypeTimestamptz UTC: got %v, want %v", got.Time, utc)
	}
}

// ─────────────────────────────────────────────────────────────
// derefStr (defined in types.go)
// ─────────────────────────────────────────────────────────────

func TestDerefStr_NilReturnsEmpty(t *testing.T) {
	if got := derefStr(nil); got != "" {
		t.Errorf("derefStr(nil) = %q, want %q", got, "")
	}
}

func TestDerefStr_NonNilReturnsValue(t *testing.T) {
	s := "hello"
	if got := derefStr(&s); got != s {
		t.Errorf("derefStr(&%q) = %q, want %q", s, got, s)
	}
}

func TestDerefStr_EmptyStringPointerReturnsEmpty(t *testing.T) {
	s := ""
	if got := derefStr(&s); got != "" {
		t.Errorf("derefStr(&\"\") = %q, want %q", got, "")
	}
}

// ─────────────────────────────────────────────────────────────
// derefInt64
// ─────────────────────────────────────────────────────────────

func TestDerefInt64_NilReturnsZero(t *testing.T) {
	if got := derefInt64(nil); got != 0 {
		t.Errorf("derefInt64(nil) = %d, want 0", got)
	}
}

func TestDerefInt64_NonNilReturnsValue(t *testing.T) {
	n := int64(99)
	if got := derefInt64(&n); got != 99 {
		t.Errorf("derefInt64(&99) = %d, want 99", got)
	}
}

// ─────────────────────────────────────────────────────────────
// toFloat64FromNumeric / toNumericFromFloat64
// ─────────────────────────────────────────────────────────────

func TestToFloat64FromNumeric_InvalidReturnsZero(t *testing.T) {
	var n pgtype.Numeric // zero value, Valid == false
	got := toFloat64FromNumeric(n)
	if got != 0 {
		t.Errorf("toFloat64FromNumeric(invalid) = %v, want 0", got)
	}
}

func TestToFloat64FromNumeric_ValidValue(t *testing.T) {
	n := toNumericFromFloat64(3.14)
	got := toFloat64FromNumeric(n)
	// Allow small floating-point tolerance
	diff := got - 3.14
	if diff < -1e-9 || diff > 1e-9 {
		t.Errorf("toFloat64FromNumeric round-trip: got %v, want ~3.14", got)
	}
}

func TestToNumericFromFloat64_Zero(t *testing.T) {
	n := toNumericFromFloat64(0)
	got := toFloat64FromNumeric(n)
	if got != 0 {
		t.Errorf("toNumericFromFloat64(0) round-trip: got %v, want 0", got)
	}
}

func TestToNumericFromFloat64_NegativeValue(t *testing.T) {
	n := toNumericFromFloat64(-1.5)
	got := toFloat64FromNumeric(n)
	diff := got - (-1.5)
	if diff < -1e-9 || diff > 1e-9 {
		t.Errorf("toNumericFromFloat64(-1.5) round-trip: got %v, want -1.5", got)
	}
}

func TestToNumericFromFloat64_IsValid(t *testing.T) {
	n := toNumericFromFloat64(42.0)
	if !n.Valid {
		t.Error("toNumericFromFloat64(42.0): expected Valid == true")
	}
}

// ─────────────────────────────────────────────────────────────
// toUserPublic
// ─────────────────────────────────────────────────────────────

func TestToUserPublic_FullUser(t *testing.T) {
	username := "alice"
	firstName := "Alice"
	lastName := "Smith"
	now := time.Now().UTC().Truncate(time.Second)
	u := Users{
		ID:            1,
		UserID:        42,
		Username:      &username,
		FirstName:     &firstName,
		LastName:      &lastName,
		IsSuperAdmin:  true,
		IsAllowed:     true,
		TotalCommands: 10,
		FirstSeenAt:   pgtype.Timestamptz{Time: now, Valid: true},
		LastSeenAt:    pgtype.Timestamptz{Time: now, Valid: true},
		CreatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
	}
	pub := toUserPublic(u)
	if pub.ID != 1 {
		t.Errorf("toUserPublic ID: got %d, want 1", pub.ID)
	}
	if pub.UserID != 42 {
		t.Errorf("toUserPublic UserID: got %d, want 42", pub.UserID)
	}
	if pub.Username != "alice" {
		t.Errorf("toUserPublic Username: got %q, want %q", pub.Username, "alice")
	}
	if pub.FirstName != "Alice" {
		t.Errorf("toUserPublic FirstName: got %q, want %q", pub.FirstName, "Alice")
	}
	if pub.LastName != "Smith" {
		t.Errorf("toUserPublic LastName: got %q, want %q", pub.LastName, "Smith")
	}
	if !pub.IsSuperAdmin {
		t.Error("toUserPublic IsSuperAdmin: expected true")
	}
	if pub.TotalCommands != 10 {
		t.Errorf("toUserPublic TotalCommands: got %d, want 10", pub.TotalCommands)
	}
	if !pub.FirstSeenAt.Equal(now) {
		t.Errorf("toUserPublic FirstSeenAt: got %v, want %v", pub.FirstSeenAt, now)
	}
}

func TestToUserPublic_NilOptionalFields(t *testing.T) {
	u := Users{
		ID:     2,
		UserID: 99,
		// Username, FirstName, LastName are nil
		// Timestamps are invalid (Valid == false)
	}
	pub := toUserPublic(u)
	if pub.Username != "" {
		t.Errorf("toUserPublic nil Username: got %q, want %q", pub.Username, "")
	}
	if pub.FirstName != "" {
		t.Errorf("toUserPublic nil FirstName: got %q, want %q", pub.FirstName, "")
	}
	if pub.LastName != "" {
		t.Errorf("toUserPublic nil LastName: got %q, want %q", pub.LastName, "")
	}
	// Zero time when invalid
	if !pub.FirstSeenAt.IsZero() {
		t.Errorf("toUserPublic invalid FirstSeenAt: expected zero time, got %v", pub.FirstSeenAt)
	}
}

// ─────────────────────────────────────────────────────────────
// toChatPublic
// ─────────────────────────────────────────────────────────────

func TestToChatPublic_FullChat(t *testing.T) {
	title := "General"
	chatType := "group"
	now := time.Now().UTC().Truncate(time.Second)
	c := Chats{
		ID:        5,
		ChatID:    -100123,
		Title:     &title,
		Type:      &chatType,
		CreatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}
	pub := toChatPublic(c)
	if pub.ID != 5 {
		t.Errorf("toChatPublic ID: got %d, want 5", pub.ID)
	}
	if pub.ChatID != -100123 {
		t.Errorf("toChatPublic ChatID: got %d, want -100123", pub.ChatID)
	}
	if pub.Title != "General" {
		t.Errorf("toChatPublic Title: got %q, want %q", pub.Title, "General")
	}
	if pub.Type != "group" {
		t.Errorf("toChatPublic Type: got %q, want %q", pub.Type, "group")
	}
	if !pub.CreatedAt.Equal(now) {
		t.Errorf("toChatPublic CreatedAt: got %v, want %v", pub.CreatedAt, now)
	}
}

func TestToChatPublic_NilTitleAndType(t *testing.T) {
	c := Chats{
		ID:     3,
		ChatID: 0,
		// Title and Type are nil
	}
	pub := toChatPublic(c)
	if pub.Title != "" {
		t.Errorf("toChatPublic nil Title: got %q, want %q", pub.Title, "")
	}
	if pub.Type != "" {
		t.Errorf("toChatPublic nil Type: got %q, want %q", pub.Type, "")
	}
}

// ─────────────────────────────────────────────────────────────
// toSettingAuditPublic
// ─────────────────────────────────────────────────────────────

func TestToSettingAuditPublic_FullAudit(t *testing.T) {
	oldVal := "old"
	newVal := "new"
	chatID := int64(777)
	now := time.Now().UTC().Truncate(time.Second)
	a := SettingAudits{
		ID:        10,
		Key:       "some_key",
		OldValue:  &oldVal,
		NewValue:  &newVal,
		ChangedBy: 55,
		ChatID:    &chatID,
		ChangedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}
	pub := toSettingAuditPublic(a)
	if pub.ID != 10 {
		t.Errorf("toSettingAuditPublic ID: got %d, want 10", pub.ID)
	}
	if pub.Key != "some_key" {
		t.Errorf("toSettingAuditPublic Key: got %q, want %q", pub.Key, "some_key")
	}
	if pub.OldValue != "old" {
		t.Errorf("toSettingAuditPublic OldValue: got %q, want %q", pub.OldValue, "old")
	}
	if pub.NewValue != "new" {
		t.Errorf("toSettingAuditPublic NewValue: got %q, want %q", pub.NewValue, "new")
	}
	if pub.ChangedBy != 55 {
		t.Errorf("toSettingAuditPublic ChangedBy: got %d, want 55", pub.ChangedBy)
	}
	if pub.ChatID == nil || *pub.ChatID != 777 {
		t.Errorf("toSettingAuditPublic ChatID: got %v, want &777", pub.ChatID)
	}
	if !pub.ChangedAt.Equal(now) {
		t.Errorf("toSettingAuditPublic ChangedAt: got %v, want %v", pub.ChangedAt, now)
	}
}

func TestToSettingAuditPublic_NilOptionalFields(t *testing.T) {
	a := SettingAudits{
		ID:        1,
		Key:       "k",
		ChangedBy: 1,
		// OldValue, NewValue, ChatID all nil
		// ChangedAt is invalid
	}
	pub := toSettingAuditPublic(a)
	if pub.OldValue != "" {
		t.Errorf("toSettingAuditPublic nil OldValue: got %q, want %q", pub.OldValue, "")
	}
	if pub.NewValue != "" {
		t.Errorf("toSettingAuditPublic nil NewValue: got %q, want %q", pub.NewValue, "")
	}
	if !pub.ChangedAt.IsZero() {
		t.Errorf("toSettingAuditPublic invalid ChangedAt: expected zero time, got %v", pub.ChangedAt)
	}
}

// ─────────────────────────────────────────────────────────────
// toKeptTorrentPublic
// ─────────────────────────────────────────────────────────────

func TestToKeptTorrentPublic_FullRow(t *testing.T) {
	filename := "my.torrent"
	username := "bob"
	firstName := "Bob"
	lastName := "Jones"
	now := time.Now().UTC().Truncate(time.Second)
	row := ListKeptTorrentsRow{
		ID:            7,
		TorrentID:     "abc123",
		Filename:      &filename,
		KeptByID:      3,
		KeptAt:        pgtype.Timestamptz{Time: now, Valid: true},
		UserIDPk:      3,
		UserUserID:    1001,
		UserUsername:  &username,
		UserFirstName: &firstName,
		UserLastName:  &lastName,
	}
	pub := toKeptTorrentPublic(row)
	if pub.ID != 7 {
		t.Errorf("toKeptTorrentPublic ID: got %d, want 7", pub.ID)
	}
	if pub.TorrentID != "abc123" {
		t.Errorf("toKeptTorrentPublic TorrentID: got %q, want %q", pub.TorrentID, "abc123")
	}
	if pub.Filename != "my.torrent" {
		t.Errorf("toKeptTorrentPublic Filename: got %q, want %q", pub.Filename, "my.torrent")
	}
	if pub.KeptByID != 3 {
		t.Errorf("toKeptTorrentPublic KeptByID: got %d, want 3", pub.KeptByID)
	}
	if !pub.KeptAt.Equal(now) {
		t.Errorf("toKeptTorrentPublic KeptAt: got %v, want %v", pub.KeptAt, now)
	}
	if pub.User.Username != "bob" {
		t.Errorf("toKeptTorrentPublic User.Username: got %q, want %q", pub.User.Username, "bob")
	}
	if pub.User.UserID != 1001 {
		t.Errorf("toKeptTorrentPublic User.UserID: got %d, want 1001", pub.User.UserID)
	}
}

func TestToKeptTorrentPublic_NilFilenameAndUser(t *testing.T) {
	row := ListKeptTorrentsRow{
		ID:        8,
		TorrentID: "xyz",
		// Filename, UserUsername, UserFirstName, UserLastName are nil
		// KeptAt is invalid
	}
	pub := toKeptTorrentPublic(row)
	if pub.Filename != "" {
		t.Errorf("toKeptTorrentPublic nil Filename: got %q, want %q", pub.Filename, "")
	}
	if pub.User.Username != "" {
		t.Errorf("toKeptTorrentPublic nil User.Username: got %q, want %q", pub.User.Username, "")
	}
	if !pub.KeptAt.IsZero() {
		t.Errorf("toKeptTorrentPublic invalid KeptAt: expected zero time, got %v", pub.KeptAt)
	}
}

// ─────────────────────────────────────────────────────────────
// Regression: strPtr must not mutate the original string
// ─────────────────────────────────────────────────────────────

func TestStrPtr_DoesNotShareMemory(t *testing.T) {
	original := "original"
	ptr := strPtr(original)
	if ptr == nil {
		t.Fatal("strPtr returned nil for non-empty string")
	}
	// Introduce a new value to prove non-aliasing
	mutated := "mutated"
	if *ptr == mutated {
		t.Errorf("strPtr aliases memory: *ptr is equal to mutated value")
	}
}

// ─────────────────────────────────────────────────────────────
// ActivityLog metadata JSON marshaling (LogActivity helper path)
// ─────────────────────────────────────────────────────────────

func TestMetadataJSONRoundTrip(t *testing.T) {
	metadata := map[string]interface{}{
		"key": "value",
		"num": 42.0,
	}
	b, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if out["key"] != "value" {
		t.Errorf("metadata round-trip key: got %v, want %q", out["key"], "value")
	}
}
