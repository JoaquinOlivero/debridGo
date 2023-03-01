package rdebrid

import (
	"bufio"
	"debridGo/config"
	"debridGo/types"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Add torrent to Real-Debrid using .magnet or .torrent files provided by sonarr/radarr. Also, return the id of the added torrent.
func AddTorrent(releaseTitle string) (string, error) {
	// Get values from configDebridGo.toml
	conf, err := config.Values()
	if err != nil {
		log.Fatalln(err)
	}

	RDapiKey := conf.DebridGo.RDapiKey

	var torrentId string // Initialization of torrentId variable which will host the id of the added torrent.

	// Check for a .magnet file to add to Real-Debrid.
	magnet, err := magnetText(releaseTitle, conf.DebridGo.DownloadDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println(".magnet file not found. Will check for .torrent file.")
		} else {
			return "", err
		}
	}
	if magnet != "" {
		// If there is a .magnet file, send it to real-debrid and return the new added torrent id.
		client := &http.Client{}
		var data = strings.NewReader(`magnet=` + magnet)
		req, err := http.NewRequest("POST", "https://api.real-debrid.com/rest/1.0/torrents/addMagnet?", data)
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+RDapiKey)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		log.Println("Adding magnet to Real Debrid.")

		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		bodyResp, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		// Get the torrent id from the user's torrents list.
		type MagnetResponseBody struct {
			Id string `json:"id"`
		}
		var magnetResponseBody MagnetResponseBody
		err = json.Unmarshal(bodyResp, &magnetResponseBody)
		if err != nil {
			return "", err
		}

		torrentId = magnetResponseBody.Id
		resp.Body.Close()
		client.CloseIdleConnections()

		return torrentId, nil
	}

	// If no .magnet file was found, check for a .torrent file to add to Real-Debrid.
	torrentFile, err := loadTorrentFile(releaseTitle, conf.DebridGo.DownloadDir)
	if err != nil {
		if os.IsNotExist(err) {
			err = errors.New(".torrent file not found. Exiting program.")
			return "", err
		} else {
			return "", err
		}
	}

	// Upload torrentFile to Real-Debrid.
	client := &http.Client{}
	req, err := http.NewRequest("PUT", "https://api.real-debrid.com/rest/1.0/torrents/addTorrent?", torrentFile)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+RDapiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	log.Println("Adding torrent to Real Debrid.")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bodyResp, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	// Get the torrent id from the user's torrents list.
	type TorrentResponseBody struct {
		Id string `json:"id"`
	}
	var torrentResponseBody TorrentResponseBody
	err = json.Unmarshal(bodyResp, &torrentResponseBody)
	if err != nil {
		return "", err
	}

	torrentId = torrentResponseBody.Id
	resp.Body.Close()
	client.CloseIdleConnections()

	return torrentId, nil
}

func TorrentInfo(torrentId string, status bool) (types.TorrentInfoResponseBody, error) {
	time.Sleep(500 * time.Millisecond)

	// Get values from configDebridGo.toml
	conf, err := config.Values()
	if err != nil {
		log.Fatalln(err)
	}

	RDapiKey := conf.DebridGo.RDapiKey

	var torrentInfoResponseBody types.TorrentInfoResponseBody

	client := &http.Client{}

	req, err := http.NewRequest("GET", "https://api.real-debrid.com/rest/1.0/torrents/info/"+torrentId, nil)
	if err != nil {
		return torrentInfoResponseBody, err
	}
	req.Header.Set("Authorization", "Bearer "+RDapiKey)

	resp, err := client.Do(req)
	if err != nil {
		return torrentInfoResponseBody, err
	}
	defer resp.Body.Close()

	bodyResp, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return torrentInfoResponseBody, err
	}
	err = json.Unmarshal(bodyResp, &torrentInfoResponseBody)
	if err != nil {
		return torrentInfoResponseBody, err
	}

	if status {
		switch torrentInfoResponseBody.Status {
		case "magnet_error", "error", "virus", "dead":
			errStr := fmt.Sprintln("received bad torrent status from Real-Debrid: ", torrentInfoResponseBody.Status)
			err := errors.New(errStr)
			return torrentInfoResponseBody, err
		case "magnet_conversion", "waiting_files_selection", "queued", "downloading", "uploading":
			log.Println("File is not ready to download. Torrent status in Real-Debrid: ", torrentInfoResponseBody.Status)
			time.Sleep(1000 * time.Millisecond)
			resp.Body.Close()
			client.CloseIdleConnections()
			TorrentInfo(torrentId, true)
			return torrentInfoResponseBody, nil
		default:
			log.Println("Files are ready to download from Real Debrid.")
		}
	}

	resp.Body.Close()
	client.CloseIdleConnections()

	return torrentInfoResponseBody, nil

}

func SelectAndDownload(addedTorrentId string) error {
	// Get values from configDebridGo.toml
	conf, err := config.Values()
	if err != nil {
		log.Fatalln(err)
	}

	RDapiKey := conf.DebridGo.RDapiKey

	// Get torrent information to select required files from it
	torrent, err := TorrentInfo(addedTorrentId, false)
	if err != nil {
		return err
	}

	// loop through files and append to "selectedFiles" only the id of video files.
	var selectedFiles []string
	for _, file := range torrent.Files {
		if filepath.Ext(file.Path) == ".mkv" || filepath.Ext(file.Path) == ".mp4" || filepath.Ext(file.Path) == ".mov" || filepath.Ext(file.Path) == ".avi" || filepath.Ext(file.Path) == ".webm" {
			selectedFiles = append(selectedFiles, strconv.Itoa(file.Id))
		}
	}

	// Select files from "TorrentInfoResponseBody" using the ids in "selectedFiles" and start downloading them in real-debrid.
	time.Sleep(250 * time.Millisecond)

	client := &http.Client{}
	log.Println("Downloading torrent in Real-Debrid.")
	var filesId = strings.NewReader(`files=` + strings.Join(selectedFiles, ","))
	req, err := http.NewRequest("POST", "https://api.real-debrid.com/rest/1.0/torrents/selectFiles/"+torrent.Id, filesId)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+RDapiKey)
	_, err = client.Do(req)
	if err != nil {
		return err
	}
	client.CloseIdleConnections()

	return nil
}

func UnrestrictLinks(restrictedLinks []string) ([]types.UnrestrictedLinkBody, error) {
	// Get values from configDebridGo.toml
	conf, err := config.Values()
	if err != nil {
		log.Fatalln(err)
	}

	RDapiKey := conf.DebridGo.RDapiKey

	var unrestrictedLinks []types.UnrestrictedLinkBody

	for _, link := range restrictedLinks {

		log.Println("Unrestricting link: " + link)

		time.Sleep(100 * time.Millisecond)
		client := &http.Client{}
		var hosterLink = strings.NewReader(`link=` + link)
		req, err := http.NewRequest("POST", "https://api.real-debrid.com/rest/1.0/unrestrict/link", hosterLink)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+RDapiKey)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		bodyResp, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var unrestrictedLinkBody types.UnrestrictedLinkBody
		err = json.Unmarshal(bodyResp, &unrestrictedLinkBody)
		if err != nil {
			return nil, err
		}

		unrestrictedLinks = append(unrestrictedLinks, unrestrictedLinkBody)

		resp.Body.Close()
		client.CloseIdleConnections()
	}
	return unrestrictedLinks, nil
}

func magnetText(releaseTitle, downloadDir string) (string, error) {
	var magnet string
	file, err := os.Open(downloadDir + "/" + releaseTitle + ".magnet")
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// optionally, resize scanner's capacity for lines over 64K, see next example
	for scanner.Scan() {
		magnet = scanner.Text()
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}
	file.Close()
	return magnet, nil
}

func loadTorrentFile(releaseTitle, downloadDir string) (*os.File, error) {
	file, err := os.Open(downloadDir + "/" + releaseTitle + ".torrent")
	if err != nil {
		return nil, err
	}

	return file, nil
}
