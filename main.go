// usr/bin/go run $0 $@ ; exit
package main

import (
	"debridGo/config"
	"debridGo/conversion"
	"debridGo/mediaServer"
	"debridGo/servarr"
	"debridGo/types"
	"encoding/json"
	"flag"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	// dir := flag.String("dir", "", "")
	// rootDir := flag.String("rootDir", "", "")
	// torrent := flag.String("torrent", "", "")
	saveDir := flag.String("saveDir", "", "")
	rdtcHash := flag.String("hash", "", "")
	// count := flag.Int64("count", 0, "")

	flag.Parse()

	ex, err := config.ExecutableDir()
	if err != nil {
		log.Fatalln(err)
	}

	f, err := os.OpenFile(ex+"/log-debridgo.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	defer f.Close()

	log.SetOutput(f)

	// Get values from configDebridGo.toml
	conf, err := config.Values()
	if err != nil {
		log.Fatalln("Could not get value from toml file: ", err)
	}

	// Get environment variables from sonarr or radarr using the custom script option and the "On Grab" trigger.
	// Save these variables in a file so they can be used when "debridGo" is executed by "rdtClient".
	var rclonePath string

	// RADARR env variables
	radarrInternalMovieID := os.Getenv("radarr_movie_id")
	movieTitle := os.Getenv("radarr_movie_title")
	movieYear := os.Getenv("radarr_movie_year")
	torrentHash := os.Getenv("radarr_download_id") // Torrent hash that comes from radarr/sonarr. Useful to match it with the torrent hash from rdtclient
	rclonePath = conf.Rclone.RemoteName + ":" + conf.Rclone.MoviesDir + "/" + movieTitle + " (" + movieYear + ")" + "/"

	// SONARR env variables
	sonarrInternalSeriesID := os.Getenv("sonarr_series_id")        // Internal ID of the series
	seriesTitle := os.Getenv("sonarr_series_title")                // Title of the series
	seriesSeasonNumber := os.Getenv("sonarr_release_seasonnumber") // Season number from release
	if torrentHash == "" {
		torrentHash = os.Getenv("sonarr_download_id") // Torrent hash that comes from radarr/sonarr. Useful to match it with the torrent hash from rdtclient
		rclonePath = conf.Rclone.RemoteName + ":" + conf.Rclone.SeriesDir + "/" + seriesTitle + "/" + "Season " + seriesSeasonNumber + "/"
	}

	// if this script was triggered from sonarr/radarr save necessary data to a JSON file.
	if torrentHash != "" {
		// convert id from string to int and set correct category.
		var id int
		var category string
		if radarrInternalMovieID != "" {
			id, _ = strconv.Atoi(radarrInternalMovieID)
			category = "radarr"
		} else {
			id, _ = strconv.Atoi(sonarrInternalSeriesID)
			category = "tv-sonarr"
		}

		// Instance of Data struct.
		data := types.DataJSON{
			TorrentHash: torrentHash,
			ID:          id,
			Category:    category,
			RclonePath:  rclonePath,
		}

		// Marshal the struct to JSON.
		jsonData, err := json.Marshal(data)
		if err != nil {
			log.Fatalln("Error marshalling JSON: ", err)
			return
		}

		err = os.WriteFile(conf.DebridGo.DownloadDir+"/"+strings.ToLower(torrentHash)+".json", jsonData, 0777)
		if err != nil {
			log.Fatalln("Error writing to file: ", err)
		}

		log.Printf("Saving data to %v.json", strings.ToLower(torrentHash))
	}

	// This section gets triggered by rdtclient when a download finishes.
	if *saveDir != "" {

		// Get the full path of video files in saveDir.
		files, err := getVideoFiles(*saveDir)
		if err != nil {
			log.Fatalln(err)
		}

		// Convert all video files in the saveDir one at a time. This will create new .mp4 video files and new .vtt subtitle files.
		for _, file := range files {
			err = conversion.Video(file)
			if err != nil {
				log.Fatalln(err)
			}
		}

		// Get values from rdtcHash.json located in rdtclient root download directory.
		jsonFile, err := os.ReadFile(conf.DebridGo.DownloadDir + "/" + *rdtcHash + ".json")
		if err != nil {
			log.Fatalln(err)
		}

		var data types.DataJSON
		err = json.Unmarshal(jsonFile, &data)
		if err != nil {
			log.Fatalln(err)
		}

		// Copy to destination using rclone.
		err = servarr.CopyToDst(*saveDir, data.RclonePath)
		if err != nil {
			log.Fatalln(err)
		}

		// Check sonarr/radarr for new added files.
		if data.Category == "tv-sonarr" {
			err = servarr.RescanSonarr(data.ID, conf.Sonarr.ApiURL, conf.Sonarr.ApiKey)
			if err != nil {
				log.Println(err)
			} else {
				log.Println("Series rescanned successfully.")
			}

		}

		if data.Category == "radarr" {
			err = servarr.RescanRadarr(data.ID, conf.Radarr.ApiURL, conf.Radarr.ApiKey, conf.Bazarr.ApiURL, conf.Bazarr.ApiKey)
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

		// Remove saveDir and json file previously created.
		err = os.RemoveAll(*saveDir)
		if err != nil {
			log.Fatalln(err)
		}
		log.Println("Removed directory: ", *saveDir)

		err = os.Remove(conf.DebridGo.DownloadDir + "/" + *rdtcHash + ".json")
		if err != nil {
			log.Fatalln(err)
		}
		log.Printf("Removed file: %v/%v.json", conf.DebridGo.DownloadDir, *rdtcHash)

		log.Println("Done. Goodbye :)")

	}
}

func getVideoFiles(saveDir string) ([]string, error) {
	var files []string

	filepath.WalkDir(saveDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && d.Name() != filepath.Base(saveDir) {
			return filepath.SkipDir
		}

		// Append files for video conversion. Only mp4 and mkv are wanted.
		if strings.HasSuffix(d.Name(), "mp4") || strings.HasSuffix(d.Name(), "mkv") {
			files = append(files, saveDir+"/"+d.Name())
		}

		return nil
	})

	return files, nil
}
