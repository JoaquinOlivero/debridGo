package types

// // CONFIG ////
type debridGo struct {
	DownloadDir string
	RDapiKey    string
}

type sonarr struct {
	ApiURL    string
	ApiKey    string
	SeriesDir string
}

type radarr struct {
	ApiURL    string
	ApiKey    string
	MoviesDir string
}

type bazarr struct {
	ApiURL string
	ApiKey string
}

type jellyseerr struct {
	ApiURL string
	ApiKey string
}

type rclone struct {
	RemoteName string
	SeriesDir  string
	MoviesDir  string
}

type emby struct {
	ApiURL string
	ApiKey string
}

type ffmpeg struct {
	Running bool
}

type TomlConfig struct {
	DebridGo   debridGo   `toml:"debridgo"`
	Sonarr     sonarr     `toml:"sonarr"`
	Radarr     radarr     `toml:"radarr"`
	Bazarr     bazarr     `toml:"bazarr"`
	Jellyseerr jellyseerr `toml:"jellyseerr"`
	Rclone     rclone     `toml:"rclone"`
	Emby       emby       `toml:"emby"`
	Ffmpeg     ffmpeg     `toml:"ffmpeg"`
}

// //// JSON file //// //
type JSON struct {
	Torrent    string `json:"torrent"`
	ID         int    `json:"id"`
	Category   string `json:"category"`
	RclonePath string `json:"rclonePath"`
}

// //////
type TorrentFile struct {
	Id   int    `json:"id"`
	Path string `json:"path"`
}

type TorrentInfoResponseBody struct {
	Id       string        `json:"id"`
	Filename string        `json:"filename"`
	Files    []TorrentFile `json:"files"`
	Status   string        `json:"status"`
	Progress int           `json:"progress"`
	Speed    int           `json:"speed"`
	Ended    string        `json:"ended"`
	Links    []string      `json:"links"`
}

type UnrestrictedLinkBody struct {
	Id       string `json:"id"`
	Filename string `json:"filename"`
	MimeType string `json:"mimeType"`
	Filesize int    `json:"filesize"`
	Link     string `json:"link"`
	Host     string `json:"host"`
	Download string `json:"download"` // Generated download link
}
