package mediaServer

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

func ScanEmby(apiUrl, apiKey string) error {

	log.Println("Sending request to refresh library to Emby in 60 seconds...")
	time.Sleep(60000 * time.Millisecond)

	log.Println("Check if emby is performing a library scan.")
	err := checkScanEmby(apiUrl, apiKey)
	if err != nil {
		return err
	}

	log.Println("Emby is ready to perform a library scan.")

	client := &http.Client{}

	req, err := http.NewRequest("POST", apiUrl+"/Library/Refresh?api_key="+apiKey, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	_, err = client.Do(req)
	if err != nil {
		return err
	}
	log.Println("Refresh request sent to Emby.")

	log.Println("Check if Emby finished the just requested library scan.")
	err = checkScanEmby(apiUrl, apiKey)
	if err != nil {
		return err
	}

	log.Println("Emby finished scanning the library.")

	return nil
}

func checkScanEmby(apiUrl, apiKey string) error {
	client := &http.Client{}
	req, err := http.NewRequest("GET", apiUrl+"/ScheduledTasks?api_key="+apiKey, nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyResp, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Get the status of the library scan.
	type EmbyTasksResponseBody struct {
		Key   string `json:"Key"`
		State string `json:"State"`
	}
	var embyTasks []EmbyTasksResponseBody
	err = json.Unmarshal(bodyResp, &embyTasks)
	if err != nil {
		return err
	}

	for _, task := range embyTasks {
		if task.Key == "RefreshLibrary" && task.State != "Idle" {
			log.Println("Emby is performing a library scan. Checking again in 10 seconds")
			resp.Body.Close()
			time.Sleep(10000 * time.Millisecond)
			checkScanEmby(apiUrl, apiKey)
			return nil
		}
	}

	resp.Body.Close()
	return nil
}

func SyncJellyseerr(apiURL, apiKey string) error {

	client := &http.Client{}
	req, err := http.NewRequest("POST", apiURL+"/settings/jobs/jellyfin-recently-added-sync/run", nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyResp, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Get the status of the library scan.
	type JellyseerrLibScanResponseBody struct {
		Running bool `json:"running"`
	}
	var jellyseerrLibScanBody JellyseerrLibScanResponseBody
	err = json.Unmarshal(bodyResp, &jellyseerrLibScanBody)
	if err != nil {
		return err
	}

	if jellyseerrLibScanBody.Running {
		log.Println("Jellyseerr full library scan is running. Checking again in 10 seconds")
		resp.Body.Close()
		time.Sleep(10000 * time.Millisecond)
		checkJellyseerrSync(apiURL, apiKey)
		return nil
	}

	resp.Body.Close()
	log.Println("Jellyseerr full library scan completed.")

	return nil
}

func checkJellyseerrSync(apiURL, apiKey string) error {
	client := &http.Client{}
	req, err := http.NewRequest("GET", apiURL+"/settings/jobs", nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyResp, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Get the status of the library scan.
	type JellyseerrJobs struct {
		Id      string `json:"id"`
		Running bool   `json:"running"`
	}
	var jellyseerrJobs []JellyseerrJobs
	err = json.Unmarshal(bodyResp, &jellyseerrJobs)
	if err != nil {
		return err
	}

	for _, job := range jellyseerrJobs {
		if job.Running && job.Id == "jellyfin-recently-added-sync" {
			log.Println("Jellyseerr full library scan is running. Checking again in 10 seconds")
			resp.Body.Close()
			time.Sleep(10000 * time.Millisecond)
			checkJellyseerrSync(apiURL, apiKey)
			return nil
		}
	}

	resp.Body.Close()
	log.Println("Jellyseerr full library scan completed.")

	return nil
}
