package db

// ActivityType represents the type of activity
type ActivityType string

const (
	ActivityTypeTorrentAdd         ActivityType = "torrent_add"
	ActivityTypeTorrentDelete      ActivityType = "torrent_delete"
	ActivityTypeTorrentInfo        ActivityType = "torrent_info"
	ActivityTypeTorrentList        ActivityType = "torrent_list"
	ActivityTypeDownloadUnrestrict ActivityType = "download_unrestrict"
	ActivityTypeDownloadList       ActivityType = "download_list"
	ActivityTypeDownloadDelete     ActivityType = "download_delete"
	ActivityTypeCommandStart       ActivityType = "command_start"
	ActivityTypeCommandHelp        ActivityType = "command_help"
	ActivityTypeCommandStatus      ActivityType = "command_status"
	ActivityTypeMagnetLink         ActivityType = "magnet_link"
	ActivityTypeHosterLink         ActivityType = "hoster_link"
	ActivityTypeCommandDashboard   ActivityType = "command_dashboard"
	ActivityTypeTorrentKeep        ActivityType = "torrent_keep"
	ActivityTypeTorrentUnkeep      ActivityType = "torrent_unkeep"
	ActivityTypeUnauthorized       ActivityType = "unauthorized"
	ActivityTypeError              ActivityType = "error"
)
