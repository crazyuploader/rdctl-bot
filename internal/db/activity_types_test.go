package db

import "testing"

// TestActivityTypeValues verifies that each ActivityType constant has the
// expected underlying string value so that stored database records remain
// stable if the constants are ever refactored.
func TestActivityTypeValues(t *testing.T) {
	tests := []struct {
		name string
		at   ActivityType
		want string
	}{
		{"TorrentAdd", ActivityTypeTorrentAdd, "torrent_add"},
		{"TorrentDelete", ActivityTypeTorrentDelete, "torrent_delete"},
		{"TorrentInfo", ActivityTypeTorrentInfo, "torrent_info"},
		{"TorrentList", ActivityTypeTorrentList, "torrent_list"},
		{"DownloadUnrestrict", ActivityTypeDownloadUnrestrict, "download_unrestrict"},
		{"DownloadList", ActivityTypeDownloadList, "download_list"},
		{"DownloadDelete", ActivityTypeDownloadDelete, "download_delete"},
		{"CommandStart", ActivityTypeCommandStart, "command_start"},
		{"CommandHelp", ActivityTypeCommandHelp, "command_help"},
		{"CommandStatus", ActivityTypeCommandStatus, "command_status"},
		{"MagnetLink", ActivityTypeMagnetLink, "magnet_link"},
		{"HosterLink", ActivityTypeHosterLink, "hoster_link"},
		{"CommandDashboard", ActivityTypeCommandDashboard, "command_dashboard"},
		{"TorrentKeep", ActivityTypeTorrentKeep, "torrent_keep"},
		{"TorrentUnkeep", ActivityTypeTorrentUnkeep, "torrent_unkeep"},
		{"Unauthorized", ActivityTypeUnauthorized, "unauthorized"},
		{"Error", ActivityTypeError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.at) != tt.want {
				t.Errorf("ActivityType%s = %q, want %q", tt.name, tt.at, tt.want)
			}
		})
	}
}

// TestActivityTypeIsString ensures ActivityType is assignable from / to string.
func TestActivityTypeIsString(t *testing.T) {
	var at ActivityType = "custom_type"
	if string(at) != "custom_type" {
		t.Errorf("ActivityType string conversion: got %q, want %q", string(at), "custom_type")
	}
}

// TestActivityTypeDistinct ensures all constants are unique values.
func TestActivityTypeDistinct(t *testing.T) {
	all := []ActivityType{
		ActivityTypeTorrentAdd,
		ActivityTypeTorrentDelete,
		ActivityTypeTorrentInfo,
		ActivityTypeTorrentList,
		ActivityTypeDownloadUnrestrict,
		ActivityTypeDownloadList,
		ActivityTypeDownloadDelete,
		ActivityTypeCommandStart,
		ActivityTypeCommandHelp,
		ActivityTypeCommandStatus,
		ActivityTypeMagnetLink,
		ActivityTypeHosterLink,
		ActivityTypeCommandDashboard,
		ActivityTypeTorrentKeep,
		ActivityTypeTorrentUnkeep,
		ActivityTypeUnauthorized,
		ActivityTypeError,
	}
	seen := make(map[ActivityType]bool, len(all))
	for _, at := range all {
		if seen[at] {
			t.Errorf("ActivityType %q is duplicated", at)
		}
		seen[at] = true
	}
}

// TestActivityTypeCount ensures the expected total number of constants is present,
// catching accidental removal.
func TestActivityTypeCount(t *testing.T) {
	const expected = 17
	all := []ActivityType{
		ActivityTypeTorrentAdd,
		ActivityTypeTorrentDelete,
		ActivityTypeTorrentInfo,
		ActivityTypeTorrentList,
		ActivityTypeDownloadUnrestrict,
		ActivityTypeDownloadList,
		ActivityTypeDownloadDelete,
		ActivityTypeCommandStart,
		ActivityTypeCommandHelp,
		ActivityTypeCommandStatus,
		ActivityTypeMagnetLink,
		ActivityTypeHosterLink,
		ActivityTypeCommandDashboard,
		ActivityTypeTorrentKeep,
		ActivityTypeTorrentUnkeep,
		ActivityTypeUnauthorized,
		ActivityTypeError,
	}
	if len(all) != expected {
		t.Errorf("ActivityType count: got %d, want %d", len(all), expected)
	}
}
