package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// toPgtypeTimestamptz converts t to a pgtype.Timestamptz with the time normalized to UTC and Valid set to true.
func toPgtypeTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

// strPtr returns a pointer to s or nil when s is an empty string.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// int64Ptr returns a pointer to n, or nil when n is zero.
func int64Ptr(n int64) *int64 {
	if n == 0 {
		return nil
	}
	return &n
}

// toUserPublic maps a sqlc Users row to a public User value.
// It converts nullable string fields using derefStr and copies nullable timestamp fields into the corresponding time.Time fields only when they are valid, returning a pointer to the constructed User.
func toUserPublic(u Users) *User {
	pub := &User{
		ID:            u.ID,
		UserID:        u.UserID,
		Username:      derefStr(u.Username),
		FirstName:     derefStr(u.FirstName),
		LastName:      derefStr(u.LastName),
		IsSuperAdmin:  u.IsSuperAdmin,
		IsAllowed:     u.IsAllowed,
		TotalCommands: u.TotalCommands,
	}
	if u.FirstSeenAt.Valid {
		pub.FirstSeenAt = u.FirstSeenAt.Time
	}
	if u.LastSeenAt.Valid {
		pub.LastSeenAt = u.LastSeenAt.Time
	}
	if u.CreatedAt.Valid {
		pub.CreatedAt = u.CreatedAt.Time
	}
	if u.UpdatedAt.Valid {
		pub.UpdatedAt = u.UpdatedAt.Time
	}
	return pub
}

// toChatPublic converts a sqlc Chats row to a public *Chat, mapping nullable string and timestamp fields to their public representations.
// Title and Type are dereferenced from nullable strings; CreatedAt and UpdatedAt are set only when the corresponding nullable timestamps are valid.
func toChatPublic(c Chats) *Chat {
	pub := &Chat{
		ID:     c.ID,
		ChatID: c.ChatID,
		Title:  derefStr(c.Title),
		Type:   derefStr(c.Type),
	}
	if c.CreatedAt.Valid {
		pub.CreatedAt = c.CreatedAt.Time
	}
	if c.UpdatedAt.Valid {
		pub.UpdatedAt = c.UpdatedAt.Time
	}
	return pub
}

// toSettingAuditPublic converts a sqlc SettingAudits row into a public SettingAudit, mapping nullable string and timestamp fields to their concrete equivalents.
func toSettingAuditPublic(a SettingAudits) SettingAudit {
	pub := SettingAudit{
		ID:        a.ID,
		Key:       a.Key,
		OldValue:  derefStr(a.OldValue),
		NewValue:  derefStr(a.NewValue),
		ChangedBy: a.ChangedBy,
		ChatID:    a.ChatID,
	}
	if a.ChangedAt.Valid {
		pub.ChangedAt = a.ChangedAt.Time
	}
	return pub
}

// toKeptTorrentPublic converts a ListKeptTorrentsRow into a KeptTorrent, mapping nested user fields and setting KeptAt when the source timestamp is valid.
func toKeptTorrentPublic(row ListKeptTorrentsRow) KeptTorrent {
	kt := KeptTorrent{
		ID:        row.ID,
		TorrentID: row.TorrentID,
		Filename:  derefStr(row.Filename),
		KeptByID:  row.KeptByID,
		User: KeptTorrentUser{
			ID:        row.UserIDPk,
			UserID:    row.UserUserID,
			Username:  derefStr(row.UserUsername),
			FirstName: derefStr(row.UserFirstName),
			LastName:  derefStr(row.UserLastName),
		},
	}
	if row.KeptAt.Valid {
		kt.KeptAt = row.KeptAt.Time
	}
	return kt
}

// toFloat64FromNumeric converts a pgtype.Numeric to a float64 and returns 0 when the numeric is not valid.
func toFloat64FromNumeric(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, _ := n.Float64Value()
	return f.Float64
}

// toNumericFromFloat64 converts a float64 to a pgtype.Numeric.
// On scan failure it returns an invalid (zero) pgtype.Numeric.
func toNumericFromFloat64(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	if err := n.Scan(fmt.Sprintf("%g", f)); err != nil {
		return pgtype.Numeric{}
	}
	return n
}

// ─────────────────────────────────────────────────────────────
// UserRepository
// ─────────────────────────────────────────────────────────────

// UserRepository handles user operations.
type UserRepository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

// NewUserRepository returns a UserRepository backed by the provided pgxpool.Pool and initialized SQLC queries.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool, queries: New(pool)}
}

// GetOrCreateUser upserts a user and returns the current record.
func (r *UserRepository) GetOrCreateUser(ctx context.Context, userID int64, username, firstName, lastName string, isSuperAdmin bool) (*User, error) {
	now := time.Now().UTC()
	u, err := r.queries.UpsertUser(ctx, UpsertUserParams{
		UserID:       userID,
		Username:     strPtr(username),
		FirstName:    strPtr(firstName),
		LastName:     strPtr(lastName),
		IsSuperAdmin: isSuperAdmin,
		FirstSeenAt:  toPgtypeTimestamptz(now),
		LastSeenAt:   toPgtypeTimestamptz(now),
		CreatedAt:    toPgtypeTimestamptz(now),
		UpdatedAt:    toPgtypeTimestamptz(now),
	})
	if err != nil {
		return nil, err
	}
	return toUserPublic(u), nil
}

// ─────────────────────────────────────────────────────────────
// ChatRepository
// ─────────────────────────────────────────────────────────────

// ChatRepository handles chat operations.
type ChatRepository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

// NewChatRepository creates a ChatRepository using the provided pgxpool.Pool and initializes the SQLC queries.
func NewChatRepository(pool *pgxpool.Pool) *ChatRepository {
	return &ChatRepository{pool: pool, queries: New(pool)}
}

// GetOrCreateChat upserts a chat and returns the current record.
func (r *ChatRepository) GetOrCreateChat(ctx context.Context, chatID int64, title, chatType string) (*Chat, error) {
	now := time.Now().UTC()
	c, err := r.queries.UpsertChat(ctx, UpsertChatParams{
		ChatID:    chatID,
		Title:     strPtr(title),
		Type:      strPtr(chatType),
		CreatedAt: toPgtypeTimestamptz(now),
		UpdatedAt: toPgtypeTimestamptz(now),
	})
	if err != nil {
		return nil, err
	}
	return toChatPublic(c), nil
}

// ─────────────────────────────────────────────────────────────
// ActivityRepository
// ─────────────────────────────────────────────────────────────

// ActivityRepository handles activity logging.
type ActivityRepository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

// NewActivityRepository returns an ActivityRepository that uses the provided pgxpool.Pool for database access.
func NewActivityRepository(pool *pgxpool.Pool) *ActivityRepository {
	return &ActivityRepository{pool: pool, queries: New(pool)}
}

// LogActivity logs a general activity.
func (r *ActivityRepository) LogActivity(ctx context.Context, requestID string, userID int64, chatID int64, username string, activityType ActivityType, command string, messageThreadID int, success bool, errorMsg string, metadata map[string]interface{}) error {
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}
	raw := json.RawMessage(metaJSON)
	var threadID *int64
	if messageThreadID != 0 {
		tid := int64(messageThreadID)
		threadID = &tid
	}
	return r.queries.InsertActivityLog(ctx, InsertActivityLogParams{
		RequestID:       strPtr(requestID),
		UserID:          userID,
		ChatID:          chatID,
		Username:        strPtr(username),
		ActivityType:    string(activityType),
		Command:         strPtr(command),
		MessageThreadID: threadID,
		Success:         success,
		ErrorMessage:    strPtr(errorMsg),
		Metadata:        &raw,
		CreatedAt:       toPgtypeTimestamptz(time.Now().UTC()),
	})
}

// ─────────────────────────────────────────────────────────────
// TorrentRepository
// ─────────────────────────────────────────────────────────────

// TorrentRepository handles torrent activity logging.
type TorrentRepository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

// NewTorrentRepository creates a TorrentRepository backed by the given pgxpool.Pool.
func NewTorrentRepository(pool *pgxpool.Pool) *TorrentRepository {
	return &TorrentRepository{pool: pool, queries: New(pool)}
}

// LogTorrentActivity logs a torrent-specific activity.
func (r *TorrentRepository) LogTorrentActivity(ctx context.Context, requestID string, userID int64, chatID int64, torrentID, torrentHash, torrentName, magnetLink, action, status string, fileSize int64, progress float64, success bool, errorMsg string, metadata map[string]interface{}) error {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}
	return r.queries.InsertTorrentActivity(ctx, InsertTorrentActivityParams{
		RequestID:     strPtr(requestID),
		UserID:        userID,
		ChatID:        chatID,
		TorrentID:     torrentID,
		TorrentHash:   strPtr(torrentHash),
		TorrentName:   strPtr(torrentName),
		MagnetLink:    strPtr(magnetLink),
		Action:        action,
		Status:        strPtr(status),
		FileSize:      int64Ptr(fileSize),
		Progress:      toNumericFromFloat64(progress),
		Success:       success,
		ErrorMessage:  strPtr(errorMsg),
		Metadata:      json.RawMessage(metaJSON),
		CreatedAt:     toPgtypeTimestamptz(time.Now().UTC()),
		SelectedFiles: json.RawMessage("[]"),
	})
}

// GetTorrentActivities retrieves torrent activities.  If userID == 0, all activities are returned.
func (r *TorrentRepository) GetTorrentActivities(ctx context.Context, userID int64, limit int) ([]TorrentActivity, error) {
	lim := int32(limit)
	if lim <= 0 {
		lim = 100
	}

	var rows []TorrentActivities
	var err error

	if userID > 0 {
		rows, err = r.queries.GetTorrentActivities(ctx, GetTorrentActivitiesParams{
			UserID: userID,
			Limit:  lim,
		})
	} else {
		rows, err = r.queries.GetAllTorrentActivities(ctx, lim)
	}

	if err != nil {
		return nil, err
	}
	result := make([]TorrentActivity, 0, len(rows))
	for _, row := range rows {
		ta := TorrentActivity{
			ID:            row.ID,
			RequestID:     derefStr(row.RequestID),
			UserID:        row.UserID,
			ChatID:        row.ChatID,
			TorrentID:     row.TorrentID,
			TorrentHash:   derefStr(row.TorrentHash),
			TorrentName:   derefStr(row.TorrentName),
			MagnetLink:    derefStr(row.MagnetLink),
			Action:        row.Action,
			Status:        derefStr(row.Status),
			FileSize:      derefInt64(row.FileSize),
			Progress:      toFloat64FromNumeric(row.Progress),
			Success:       row.Success,
			ErrorMessage:  derefStr(row.ErrorMessage),
			Metadata:      string(row.Metadata),
			SelectedFiles: string(row.SelectedFiles),
		}
		if row.CreatedAt.Valid {
			ta.CreatedAt = row.CreatedAt.Time
		}
		result = append(result, ta)
	}
	return result, nil
}

// derefInt64 returns 0 when n is nil and otherwise the value pointed to by n.
func derefInt64(n *int64) int64 {
	if n == nil {
		return 0
	}
	return *n
}

// ─────────────────────────────────────────────────────────────
// DownloadRepository
// ─────────────────────────────────────────────────────────────

// DownloadRepository handles download activity logging.
type DownloadRepository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

// NewDownloadRepository constructs a DownloadRepository backed by the provided pgxpool.Pool and initialized SQLC Queries.
func NewDownloadRepository(pool *pgxpool.Pool) *DownloadRepository {
	return &DownloadRepository{pool: pool, queries: New(pool)}
}

// LogDownloadActivity logs a download/unrestrict activity.
func (r *DownloadRepository) LogDownloadActivity(ctx context.Context, requestID string, userID int64, chatID int64, downloadID, originalLink, fileName, host, action string, fileSize int64, success bool, errorMsg string, metadata map[string]interface{}, torrentActivityID *int64) error {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}
	raw := json.RawMessage(metaJSON)
	return r.queries.InsertDownloadActivity(ctx, InsertDownloadActivityParams{
		RequestID:         strPtr(requestID),
		UserID:            userID,
		ChatID:            chatID,
		DownloadID:        strPtr(downloadID),
		OriginalLink:      strPtr(originalLink),
		FileName:          strPtr(fileName),
		FileSize:          int64Ptr(fileSize),
		Host:              strPtr(host),
		Action:            action,
		Success:           success,
		ErrorMessage:      strPtr(errorMsg),
		Metadata:          &raw,
		CreatedAt:         toPgtypeTimestamptz(time.Now().UTC()),
		TorrentActivityID: torrentActivityID,
	})
}

// ─────────────────────────────────────────────────────────────
// CommandRepository
// ─────────────────────────────────────────────────────────────

// CommandRepository handles command logging.
type CommandRepository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

// NewCommandRepository creates a CommandRepository that uses the provided pgxpool.Pool for database operations.
func NewCommandRepository(pool *pgxpool.Pool) *CommandRepository {
	return &CommandRepository{pool: pool, queries: New(pool)}
}

// LogCommand logs a command execution and atomically increments total_commands.
func (r *CommandRepository) LogCommand(ctx context.Context, userID int64, chatID int64, username, command, fullCommand string, messageThreadID int, executionTime int64, success bool, errorMsg string, responseLength int) error {
	var threadID *int64
	if messageThreadID != 0 {
		tid := int64(messageThreadID)
		threadID = &tid
	}
	respLen := int64(responseLength)

	return withTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := New(tx)
		if err := q.InsertCommandLog(ctx, InsertCommandLogParams{
			UserID:          userID,
			ChatID:          chatID,
			Username:        strPtr(username),
			Command:         command,
			FullCommand:     strPtr(fullCommand),
			MessageThreadID: threadID,
			ExecutionTime:   &executionTime,
			Success:         success,
			ErrorMessage:    strPtr(errorMsg),
			ResponseLength:  &respLen,
			CreatedAt:       toPgtypeTimestamptz(time.Now().UTC()),
		}); err != nil {
			return err
		}
		return q.IncrementUserCommands(ctx, userID)
	})
}

// GetUserStats retrieves user statistics by internal user ID.
// All four queries run inside a REPEATABLE READ transaction so the snapshot is
// consistent even when concurrent writes are in flight.
func (r *CommandRepository) GetUserStats(ctx context.Context, userID int64) (map[string]interface{}, error) {
	var stats map[string]interface{}
	err := withReadTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := New(tx)

		u, err := q.GetUserByID(ctx, userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("user not found")
			}
			return err
		}

		totalActivities, err := q.CountActivitiesByUser(ctx, userID)
		if err != nil {
			return err
		}
		totalTorrents, err := q.CountTorrentAddsByUser(ctx, userID)
		if err != nil {
			return err
		}
		totalDownloads, err := q.CountDownloadsByUser(ctx, userID)
		if err != nil {
			return err
		}

		var firstSeen, lastSeen time.Time
		if u.FirstSeenAt.Valid {
			firstSeen = u.FirstSeenAt.Time
		}
		if u.LastSeenAt.Valid {
			lastSeen = u.LastSeenAt.Time
		}

		stats = map[string]interface{}{
			"total_commands":   u.TotalCommands,
			"total_activities": totalActivities,
			"total_torrents":   totalTorrents,
			"total_downloads":  totalDownloads,
			"first_seen_at":    firstSeen,
			"last_seen_at":     lastSeen,
		}
		return nil
	})
	return stats, err
}

// ─────────────────────────────────────────────────────────────
// SettingRepository
// ─────────────────────────────────────────────────────────────

// SettingRepository handles runtime configuration settings.
type SettingRepository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

// NewSettingRepository creates a SettingRepository bound to the provided pgxpool.Pool.
func NewSettingRepository(pool *pgxpool.Pool) *SettingRepository {
	return &SettingRepository{pool: pool, queries: New(pool)}
}

// GetSetting retrieves a setting value by key.  Returns "" if not found.
func (r *SettingRepository) GetSetting(ctx context.Context, key string) (string, error) {
	s, err := r.queries.GetSetting(ctx, key)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return s.Value, nil
}

// SetSetting creates or updates a setting value.
func (r *SettingRepository) SetSetting(ctx context.Context, key, value string) error {
	return r.queries.UpsertSetting(ctx, UpsertSettingParams{
		Key:       key,
		Value:     value,
		UpdatedAt: toPgtypeTimestamptz(time.Now().UTC()),
	})
}

// SetSettingWithAudit creates/updates a setting and writes an audit record.
// changedByUserID is the Telegram user_id of the actor; it is resolved to the
// internal users.id inside the transaction so the FK is consistent with all
// other tables. chatPK is the internal chats.id (FK to chats(id)).
func (r *SettingRepository) SetSettingWithAudit(ctx context.Context, key, value string, changedByUserID int64, chatPK int64) error {
	now := time.Now().UTC()
	return withTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := New(tx)

		// Resolve Telegram user_id → internal users.id (consistent with all other FK references).
		actor, err := q.GetUserByUserID(ctx, changedByUserID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("actor user %d not found", changedByUserID)
			}
			return fmt.Errorf("failed to load actor user %d: %w", changedByUserID, err)
		}

		// Read old value inside the transaction.
		oldValue := ""
		existing, err := q.GetSetting(ctx, key)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil {
			oldValue = existing.Value
		}

		if err := q.UpsertSetting(ctx, UpsertSettingParams{
			Key:       key,
			Value:     value,
			UpdatedAt: toPgtypeTimestamptz(now),
		}); err != nil {
			return err
		}

		var chatIDPtr *int64
		if chatPK != 0 {
			chatIDPtr = &chatPK
		}
		return q.InsertSettingAudit(ctx, InsertSettingAuditParams{
			Key:       key,
			OldValue:  strPtr(oldValue),
			NewValue:  strPtr(value),
			ChangedBy: actor.ID,
			ChatID:    chatIDPtr,
			ChangedAt: toPgtypeTimestamptz(now),
		})
	})
}

// GetSettingHistory returns audit records for a setting key.
func (r *SettingRepository) GetSettingHistory(ctx context.Context, key string, limit int) ([]SettingAudit, error) {
	lim := int32(limit)
	if lim <= 0 {
		lim = 100
	}
	rows, err := r.queries.GetSettingHistory(ctx, GetSettingHistoryParams{Key: key, Limit: lim})
	if err != nil {
		return nil, err
	}
	result := make([]SettingAudit, 0, len(rows))
	for _, row := range rows {
		result = append(result, toSettingAuditPublic(row))
	}
	return result, nil
}

// ─────────────────────────────────────────────────────────────
// KeptTorrentRepository
// ─────────────────────────────────────────────────────────────

// KeptTorrentRepository handles kept torrent operations.
type KeptTorrentRepository struct {
	pool    *pgxpool.Pool
	queries *Queries
}

// NewKeptTorrentRepository creates a KeptTorrentRepository backed by the provided pgxpool.Pool.
func NewKeptTorrentRepository(pool *pgxpool.Pool) *KeptTorrentRepository {
	return &KeptTorrentRepository{pool: pool, queries: New(pool)}
}

// KeepTorrent marks a torrent as kept.  If maxKept > 0, the limit is enforced
// atomically inside a transaction.
func (r *KeptTorrentRepository) KeepTorrent(ctx context.Context, torrentID, filename string, keptByID int64, maxKept int) error {
	now := time.Now().UTC()

	return withTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := New(tx)

		// Lock the user row to prevent concurrent keep races.
		user, err := q.LockUserForUpdate(ctx, keptByID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("cannot keep torrent: actor user %d not found", keptByID)
			}
			return fmt.Errorf("failed to load actor user %d: %w", keptByID, err)
		}

		// Atomic limit check.
		if maxKept > 0 {
			count, err := q.CountKeptExcluding(ctx, CountKeptExcludingParams{
				KeptByID:  user.ID,
				TorrentID: torrentID,
			})
			if err != nil {
				return fmt.Errorf("failed to count kept torrents: %w", err)
			}
			if count >= int64(maxKept) {
				return fmt.Errorf("maximum kept torrent limit (%d) reached", maxKept)
			}
		}

		if err := q.UpsertKeptTorrent(ctx, UpsertKeptTorrentParams{
			TorrentID: torrentID,
			Filename:  strPtr(filename),
			KeptByID:  user.ID,
			KeptAt:    toPgtypeTimestamptz(now),
		}); err != nil {
			return err
		}

		return q.InsertKeptTorrentAction(ctx, InsertKeptTorrentActionParams{
			TorrentID: torrentID,
			Action:    "keep",
			UserID:    user.ID,
			Username:  user.Username,
			CreatedAt: toPgtypeTimestamptz(now),
		})
	})
}

// UnkeepTorrent removes the keep mark from a torrent.
func (r *KeptTorrentRepository) UnkeepTorrent(ctx context.Context, torrentID string, unkeptByID int64, isAdmin bool) error {
	now := time.Now().UTC()

	return withTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := New(tx)

		user, err := q.GetUserByUserID(ctx, unkeptByID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("cannot unkeep torrent: actor user %d not found", unkeptByID)
			}
			return fmt.Errorf("failed to load actor user %d: %w", unkeptByID, err)
		}

		if isAdmin {
			tag, err := tx.Exec(ctx,
				"DELETE FROM kept_torrents WHERE torrent_id = $1",
				torrentID)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return fmt.Errorf("torrent is not kept or you don't have permission to unkeep it")
			}
		} else {
			tag, err := tx.Exec(ctx,
				"DELETE FROM kept_torrents WHERE torrent_id = $1 AND kept_by_id = $2",
				torrentID, user.ID)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return fmt.Errorf("torrent is not kept or you don't have permission to unkeep it")
			}
		}

		return q.InsertKeptTorrentAction(ctx, InsertKeptTorrentActionParams{
			TorrentID: torrentID,
			Action:    "unkeep",
			UserID:    user.ID,
			Username:  user.Username,
			CreatedAt: toPgtypeTimestamptz(now),
		})
	})
}

// IsKept checks if any user has marked the torrent as kept.
func (r *KeptTorrentRepository) IsKept(ctx context.Context, torrentID string) (bool, error) {
	_, err := r.queries.CheckKeptTorrent(ctx, torrentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetKeptTorrentIDs returns a map of all kept torrent IDs for quick lookup.
func (r *KeptTorrentRepository) GetKeptTorrentIDs(ctx context.Context) (map[string]bool, error) {
	ids, err := r.queries.GetAllKeptTorrentIDs(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result, nil
}

// ListKeptTorrents returns all kept torrents with associated user details.
func (r *KeptTorrentRepository) ListKeptTorrents(ctx context.Context) ([]KeptTorrent, error) {
	rows, err := r.queries.ListKeptTorrents(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]KeptTorrent, 0, len(rows))
	for _, row := range rows {
		result = append(result, toKeptTorrentPublic(row))
	}
	return result, nil
}

// CountKeptByUser returns the number of torrents kept by a user (by Telegram user_id).
func (r *KeptTorrentRepository) CountKeptByUser(ctx context.Context, userID int64) (int64, error) {
	u, err := r.queries.GetUserByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return r.queries.CountKeptByUser(ctx, u.ID)
}

// ─────────────────────────────────────────────────────────────
// transaction helper
// withTx begins a transaction on the provided pool, executes fn with the started transaction, rolls back if fn returns an error, and commits on success.
// It returns the error returned by fn or any error encountered while committing the transaction.

func withTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

// withReadTx runs fn inside a REPEATABLE READ read-only transaction so that
// multiple SELECT statements see a consistent snapshot.
func withReadTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
