package main

import (
	"debridGo/config"
	"debridGo/conversion"
	"debridGo/download"
	"debridGo/mediaServer"
	"debridGo/rdebrid"
	"debridGo/servarr"
	"log"
	"os"
	"strings"
	"time"
)

// Get the torrent id from the user's torrents list.

func main() {
	// Get values from configDebridGo.toml
	conf, err := config.Values()
	if err != nil {
		log.Fatalln(err)
	}

	var downloadPath string

	// RADARR env variables
	radarrInternalMovieID := os.Getenv("radarr_movie_id")
	movieTitle := os.Getenv("radarr_movie_title")
	movieYear := os.Getenv("radarr_movie_year")
	releaseTitle := os.Getenv("radarr_release_title")
	downloadPath = movieTitle + " (" + movieYear + ")" + "/"

	// SONARR env variables
	sonarrInternalSeriesID := os.Getenv("sonarr_series_id")        // Internal ID of the series
	seriesTitle := os.Getenv("sonarr_series_title")                // Title of the series
	seriesSeasonNumber := os.Getenv("sonarr_release_seasonnumber") // Season number from release
	// seriesEpisodeNumber := os.Getenv("sonarr_release_episodenumbers") // Comma-delimitied list of episode numbers
	if releaseTitle == "" {
		releaseTitle = os.Getenv("sonarr_release_title")
		downloadPath = seriesTitle + "/" + "Season " + seriesSeasonNumber + "/"
	}

	ex, err := config.ExecutableDir()
	if err != nil {
		log.Fatalln(err)
	}

	tempDownloadDirectory := conf.DebridGo.DownloadDir + "/" + downloadPath
	downloadPath = strings.ReplaceAll(downloadPath, ":", "") // remove characters incompatible with radarr's amd sonarr's directory naming conventions.

	if releaseTitle != "" {
		// Log to file
		os.MkdirAll(ex+"/debridGo/logs/"+downloadPath, 0777)

		f, err := os.OpenFile(ex+"/debridGo/logs/"+downloadPath+"/"+releaseTitle+".log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}

		defer f.Close()

		log.SetOutput(f)
		log.Println("release title: " + releaseTitle)

		log.Printf("Received %v download request.\n", releaseTitle)

		// Add magnet/torrent to Real-Debrid.
		addedTorrentId, err := rdebrid.AddTorrent(releaseTitle)
		if err != nil {
			log.Fatalln(err)
		}

		// Select files from the added torrent and send request to download them in Real-Debrid.
		err = rdebrid.SelectAndDownload(addedTorrentId)
		if err != nil {
			log.Fatalln(err)
		}

		// Check whether the files downloaded in Real-Debrid from the torrent, are available for this client to download using "TorrentInfoResponseBody.Id".
		time.Sleep(250 * time.Millisecond)
		torrent, err := rdebrid.TorrentInfo(addedTorrentId, true)
		if err != nil {
			log.Fatalln(err)
		}

		// Unrestrict original hoster links and get download links.
		time.Sleep(250 * time.Millisecond)
		apiLinks, err := rdebrid.UnrestrictLinks(torrent.Links)
		if err != nil {
			log.Fatalln(err)
		}

		// Download file.
		err = download.DownloadFromDebrid(apiLinks, tempDownloadDirectory)
		if err != nil {
			log.Fatalln(err)
		}

		// Convert one video file at a time and get the new filepath back.
		for _, file := range apiLinks {
			err := conversion.Video(tempDownloadDirectory + file.Filename)
			if err != nil {
				log.Fatalln(err)
				return
			}
		}

		// Move video files to destination directory.
		var rcloneDstDir string
		if sonarrInternalSeriesID != "" {
			downloadPath = strings.ReplaceAll(downloadPath, ":", "")
			rcloneDstDir = conf.Rclone.RemoteName + ":" + conf.Rclone.SeriesDir + "/" + downloadPath
		}

		if radarrInternalMovieID != "" {
			rcloneDstDir = conf.Rclone.RemoteName + ":" + conf.Rclone.MoviesDir + "/" + downloadPath
		}
		for _, file := range apiLinks {
			err = servarr.CopyToDst(file.Filename, releaseTitle, tempDownloadDirectory, rcloneDstDir)
			if err != nil {
				log.Fatalln(err)
				return
			}
		}

		// Rescan radarr and sonarr.
		if sonarrInternalSeriesID != "" {
			err = servarr.StopEpisodeSearch(releaseTitle, conf.Sonarr.ApiURL, conf.Sonarr.ApiKey)
			if err != nil {
				log.Println(err)
			}

			err = servarr.RescanSonarr(sonarrInternalSeriesID, conf.Sonarr.ApiURL, conf.Sonarr.ApiKey)
			if err != nil {
				log.Println(err)
			}
			log.Println("Series rescanned successfully.")
		}

		if radarrInternalMovieID != "" {
			err = servarr.RescanRadarr(radarrInternalMovieID, conf.Radarr.ApiURL, conf.Radarr.ApiKey, conf.Bazarr.ApiURL, conf.Bazarr.ApiKey)
			if err != nil {
				log.Fatalln(err)
			}
			log.Println("Movie rescanned successfully.")
		}

		// Once everything is ready and where it belongs, send a request to emby/jellyfin to scan the library. And then to Jellyseerr
		err = mediaServer.ScanEmby(conf.Emby.ApiURL, conf.Emby.ApiKey)
		if err != nil {
			log.Fatalln(err)
		}

		// Sync Jellyseerr.
		err = mediaServer.SyncJellyseerr(conf.Jellyseerr.ApiURL, conf.Jellyseerr.ApiKey)
		if err != nil {
			log.Fatalln(err)
		}

		log.Println("Done. Goodbye :)")
		return
	}

}
