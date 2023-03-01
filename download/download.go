package download

import (
	"debridGo/types"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cavaliergopher/grab/v3"
)

func DownloadFromDebrid(apiLinks []types.UnrestrictedLinkBody, tempDownloadDirectory string) error {
	log.Println("Starting download from Real Debrid.")

	// Create the directory and file.
	err := os.MkdirAll(tempDownloadDirectory, os.ModePerm)
	if err != nil {
		return err
	}

	reqs := make([]*grab.Request, 0)
	for _, link := range apiLinks {
		apiLink := strings.TrimLeft(link.Download, "htps:/")
		splitApiLink := strings.Split(apiLink, "/")                    // split url between "/"
		downloadLinkServerId := strings.Split(splitApiLink[0], ".")[0] // Gets the server id where the file is hosted.
		downloadLinkHostServerIds := strings.Replace(splitApiLink[2], splitApiLink[2], splitApiLink[2]+downloadLinkServerId, -1)

		downloadLink := "https://sao1" + splitApiLink[0][len(downloadLinkServerId):] + "/" + splitApiLink[1] + "/" + downloadLinkHostServerIds + "/" + splitApiLink[3]
		log.Println("Download link: " + downloadLink)
		req, err := grab.NewRequest(tempDownloadDirectory, downloadLink)
		if err != nil {
			return err
		}
		reqs = append(reqs, req)
	}

	// start downloads with 4 workers
	client := grab.NewClient()
	respch := client.DoBatch(4, reqs...)

	// start UI loop
	t := time.NewTicker(1000 * time.Millisecond)

	// check each response
	// monitor downloads
	completed := 0
	inProgress := 0
	responses := make([]*grab.Response, 0)
	for completed < len(apiLinks) {
		select {
		case resp := <-respch:
			// a new response has been received and has started downloading
			// (nil is received once, when the channel is closed by grab)
			if resp != nil {
				responses = append(responses, resp)
			}

		case <-t.C:
			// update completed downloads
			for i, resp := range responses {
				if resp != nil && resp.IsComplete() {
					// print final result
					if resp.Err() != nil {
						errString := fmt.Sprintf("Error downloading %s: %v\n", resp.Request.URL(), resp.Err())
						err = errors.New(errString)
						return err
					} else {
						log.Printf("Finished %s %v / %v Mb (%d%%)\n", filepath.Base(resp.Filename), (resp.BytesComplete())/(1000*1000), (resp.Size())/(1000*1000), int(100*resp.Progress()))
					}

					// mark completed
					responses[i] = nil
					completed++
				}
			}

			// update downloads in progress
			inProgress = 0
			for _, resp := range responses {
				if resp != nil {
					inProgress++
					log.Printf("Downloading %s %v / %v Mb (%d%%) ---- %.2f MB/s \n", filepath.Base(resp.Filename), (resp.BytesComplete())/(1000*1000), (resp.Size())/(1000*1000), int(100*resp.Progress()), (resp.BytesPerSecond())/(1000*1000))
				}
			}
		}
	}

	t.Stop()

	log.Printf("%d files successfully downloaded.\n", len(apiLinks))

	return nil
}
