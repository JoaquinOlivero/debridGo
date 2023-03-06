package servarr

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"sync"

	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Copy to destination using rclone.
func CopyToDst(filename, tempDownloadDirectory, rcloneDstDir string, wg *sync.WaitGroup) error {
	defer wg.Done()
	filename = strings.TrimSuffix(filename, filepath.Ext(filename))

	// Match all files corresponding to the downloaded and converted video file.
	files, err := walkMatch(tempDownloadDirectory, filename+".*")
	if err != nil {
		return err
	}

	for _, file := range files {
		time.Sleep(250 * time.Millisecond)
		// Execute rclone copy command
		args := []string{"copy", file, rcloneDstDir, "--fast-list", "-P"}
		var errb bytes.Buffer
		cmd := exec.Command("rclone", args...)
		cmd.Dir = "/"
		cmd.Stderr = &errb
		cmd.Start()
		// Log stuff...
		log.Printf("rclone arguments: %v\n", cmd.Args)
		cmd.Wait()

		log.Println("rclone error:", errb.String())

		time.Sleep(500 * time.Millisecond)
		os.Remove(file)
	}

	return nil
}

func walkMatch(root, pattern string) ([]string, error) {
	var matches []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if matched, err := filepath.Match(pattern, filepath.Base(path)); err != nil {
			return err
		} else if matched {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}

func RescanSonarr(seriesSonarrId int, sonarrApiUrl, sonarrApiKey string) error {
	log.Println("Refreshing series in sonarr")
	client := &http.Client{}

	type Body struct {
		Name     string `json:"name"`
		SeriesId int    `json:"seriesId"`
	}

	postBody := Body{
		Name:     "RescanSeries",
		SeriesId: seriesSonarrId,
	}

	postBodyJSON, err := json.Marshal(postBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", sonarrApiUrl+"/command/?apikey="+sonarrApiKey, bytes.NewBuffer(postBodyJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	bodyResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Get the id of the recently executed command and check its status
	type SonarrRescanResponseBody struct {
		CommandId int `json:"id"`
	}
	var sonarrRescanBody SonarrRescanResponseBody
	err = json.Unmarshal(bodyResp, &sonarrRescanBody)
	if err != nil {
		return err
	}

	time.Sleep(5000 * time.Millisecond)
	err = checkSonarrRescanStatus(sonarrRescanBody.CommandId, sonarrApiUrl, sonarrApiKey)
	if err != nil {
		return err
	}

	return nil
}

func checkSonarrRescanStatus(commandId int, sonarrApiUrl, sonarrApiKey string) error {
	log.Println("Checking recent Sonarr rescan status.")
	client := &http.Client{}
	commandIdStr := strconv.Itoa(commandId)
	req, err := http.NewRequest("GET", sonarrApiUrl+"/command/"+commandIdStr+"/"+"?apikey="+sonarrApiKey, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	bodyResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Get the id of the recently executed command and check its status
	type SonarrRescanCheckStatusResponseBody struct {
		Status string `json:"status"`
	}
	var sonarrRescanCheckStatusBody SonarrRescanCheckStatusResponseBody
	err = json.Unmarshal(bodyResp, &sonarrRescanCheckStatusBody)
	if err != nil {
		return err
	}

	if sonarrRescanCheckStatusBody.Status == "failed" {
		err = errors.New("sonarr rescan failed")
		return err
	}

	if sonarrRescanCheckStatusBody.Status != "completed" {
		log.Printf("Rescan status: %v", sonarrRescanCheckStatusBody.Status)
		log.Println("Rescan not completed. Checking again in 5s.")
		time.Sleep(5000 * time.Millisecond)
		checkSonarrRescanStatus(commandId, sonarrApiUrl, sonarrApiKey)
		return nil
	}
	return nil
}

func RescanRadarr(movieRadarrId int, radarrApiUrl, radarrApiKey, bazarrApiURL, bazarrApiKey string) error {
	log.Println("Refreshing movie in radarr")
	client := &http.Client{}

	type Body struct {
		Name    string `json:"name"`
		MovieId int    `json:"movieId"`
	}

	postBody := Body{
		Name:    "RescanMovie",
		MovieId: movieRadarrId,
	}

	postBodyJSON, err := json.Marshal(postBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", radarrApiUrl+"/command/?apikey="+radarrApiKey, bytes.NewBuffer(postBodyJSON))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	bodyResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Get the id of the recently executed command and check its status
	type RadarrRescanResponseBody struct {
		CommandId int `json:"id"`
	}
	var radarrRescanBody RadarrRescanResponseBody
	err = json.Unmarshal(bodyResp, &radarrRescanBody)
	if err != nil {
		return err
	}

	time.Sleep(5000 * time.Millisecond)
	err = checkRadarrRescanStatus(radarrRescanBody.CommandId, radarrApiUrl, radarrApiKey)
	if err != nil {
		return err
	}

	// Send req to bazarr to search for subtitles.

	bazarrBodyData := strings.NewReader("radarr_moviefile_id=" + strconv.Itoa(movieRadarrId))
	req, err = http.NewRequest("POST", bazarrApiURL+"/radarr", bazarrBodyData)
	if err != nil {
		return err

	}
	req.Header.Set("x-api-key", bazarrApiKey)
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	_, err = client.Do(req)
	if err != nil {
		return err
	}
	time.Sleep(5000 * time.Millisecond)
	return nil
}

func checkRadarrRescanStatus(commandId int, radarrApiUrl, radarrApiKey string) error {
	log.Println("Checking recent Radarr rescan status.")
	client := &http.Client{}
	commandIdStr := strconv.Itoa(commandId)
	req, err := http.NewRequest("GET", radarrApiUrl+"/command/"+commandIdStr+"/"+"?apikey="+radarrApiKey, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	bodyResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Get the id of the recently executed command and check its status
	type RadarrRescanCheckStatusResponseBody struct {
		Status string `json:"status"`
	}
	var radarrRescanCheckStatusBody RadarrRescanCheckStatusResponseBody
	err = json.Unmarshal(bodyResp, &radarrRescanCheckStatusBody)
	if err != nil {
		return err
	}

	if radarrRescanCheckStatusBody.Status == "failed" {
		err = errors.New("radarr rescan failed")
		return err
	}

	if radarrRescanCheckStatusBody.Status != "completed" {
		log.Printf("Rescan status: %v", radarrRescanCheckStatusBody.Status)
		log.Println("Rescan not completed. Checking again in 5s.")
		time.Sleep(5000 * time.Millisecond)
		checkRadarrRescanStatus(commandId, radarrApiUrl, radarrApiKey)
		return nil
	}

	return nil
}

func StopEpisodeSearch(releaseTitle, apiURL, apiKey string) error {
	type Body struct {
		Name    string `json:"name"`
		Message string `json:"message"`
		Status  string `json:"status"`
		Id      int    `json:"id"`
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", apiURL+"/command/?apikey="+apiKey, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	var body []Body
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		return err
	}

	resp.Body.Close()

	// Stop and remove all the episode search tasks.
	var commandIds []int

	for _, task := range body {
		if task.Name == "EpisodeSearch" {
			commandIds = append(commandIds, task.Id)
		}
	}
	if len(commandIds) > 0 {
		for _, id := range commandIds {
			log.Println(`Stopping command "EpisodeSearch" with id: ` + strconv.Itoa(id))
			req, err := http.NewRequest("DELETE", apiURL+"/command/"+strconv.Itoa(id)+"?apikey="+apiKey, nil)
			if err != nil {
				return err
			}

			resp, err := client.Do(req)
			if err != nil {
				return err
			}

			log.Println("Delete request to stop EpisodeSearch returned: " + resp.Status)

		}
	}

	return nil
}

// [
//     {
//         "name": "EpisodeSearch",
//         "commandName": "Episode Search",
//         "message": "Report sent to rd. Tulsa.King.S01E05.Token.Joe.1080p.AMZN.WEBRip.DDP5.1.x264-NTb[rartv]",
//         "body": {
//             "episodeIds": [
//                 1660
//             ],
//             "sendUpdatesToClient": true,
//             "updateScheduledTask": true,
//             "completionMessage": "Completed",
//             "requiresDiskAccess": false,
//             "isExclusive": false,
//             "name": "EpisodeSearch",
//             "trigger": "manual",
//             "suppressMessages": false,
//             "clientUserAgent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36"
//         },
//         "priority": "normal",
//         "status": "started",
//         "queued": "2023-02-21T03:22:35.309555Z",
//         "started": "2023-02-21T03:22:35.316204Z",
//         "trigger": "manual",
//         "stateChangeTime": "2023-02-21T03:22:35.316204Z",
//         "sendUpdatesToClient": true,
//         "updateScheduledTask": true,
//         "id": 253108
//     }
// ]
