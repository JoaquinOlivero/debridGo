package servarr

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Copy to destination using rclone.
func CopyToDst(saveDir, rcloneDstDir string) error {
	err := removeUnwanted(saveDir)
	if err != nil {
		return err
	}

	// Execute rclone copy command
	args := []string{"copy", saveDir, rcloneDstDir, "-P", "--transfers", "3"}
	var errb bytes.Buffer
	cmd := exec.Command("rclone", args...)
	cmd.Dir = "/"
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = &errb
	cmd.Start()
	// Log stuff...
	log.Printf("rclone arguments: %v\n", cmd.Args)

	// Log upload status. A lot of lines written into the log file...
	scanner := bufio.NewScanner(stdout)

	// Parse? the stdout from the command executing "rclone copy" to make it readable and only get the upload speed, percentage and ETA from the stdout.
	var info string // Clean upload status line.
	for scanner.Scan() {
		var a []string
		m := scanner.Text()

		// The code below is basically parsing the stdout to only get the desired data.
		if strings.Contains(m, "Transferred:") && strings.Contains(m, "MiB/s, ETA") {
			_, after, _ := strings.Cut(m, "Transferred:")
			// remove the unnecessary empty spaces.
			split := strings.Split(after, " ")

			for i := 0; i < len(split); i++ {
				if split[i] != " " && i > 3 {

					a = append(a, split[i])
					info = strings.Join(a, " ")
				}
			}
		}
		log.Println(info) // log the readable data.
	}
	cmd.Wait()

	log.Println("rclone error:", errb.String())
	log.Println("rclone finished: ", filepath.Base(saveDir))

	time.Sleep(500 * time.Millisecond)
	return nil
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

func removeUnwanted(saveDir string) error {

	filepath.WalkDir(saveDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip and remove subdirectories that usually have subs or other unwanted files.
		if d.IsDir() && d.Name() != filepath.Base(saveDir) {
			os.RemoveAll(saveDir + "/" + d.Name())
			return filepath.SkipDir
		}

		// Remove unwanted files.
		if !d.IsDir() && !strings.HasSuffix(d.Name(), ".mp4") && !strings.HasSuffix(d.Name(), ".vtt") {
			os.Remove(saveDir + "/" + d.Name())
		}

		return nil
	})

	return nil
}
