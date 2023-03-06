package conversion

import (
	"debridGo/config"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

// Decoding Movie data into JSON
type VideoFileInfoProbe struct {
	Streams []struct {
		Index       int    `json:"index"`
		CodecType   string `json:"codec_type"`
		CodecName   string `json:"codec_name"`
		Channels    int    `json:"channels"`
		Bitrate     string `json:"bit_rate"`
		Disposition struct {
			Default int `json:"default"`
		} `json:"disposition"`
		Tags struct {
			Language    string `json:"language"`
			HandlerName string `json:"handler_name"`
			Title       string `json:"title"`
		} `json:"tags"`
	} `json:"streams"`
}

func Video(filePath string) error {
	conf, err := config.Values()
	if err != nil {
		return err
	}

	if conf.Ffmpeg.Running {
		log.Println("Ffmpeg is running. Can't proceed. Trying again in 20 seconds.")
		time.Sleep(20000 * time.Millisecond)
		Video(filePath)
		return nil
	}

	// Set ffmpeg running to true in config file
	config.SetFfmpeg(true)

	// Convert single video file.
	log.Println("Obtainig file information for video conversion.")
	data, err := ffmpeg.Probe(filePath)
	if err != nil {
		return err
	}

	err = convert(data, filePath)
	if err != nil {
		return err
	}

	// Set ffmpeg running to false in config file
	config.SetFfmpeg(false)

	return nil
}

func convert(fileData string, filePath string) error {

	var (
		totalAudioStreams       int
		audioStreamIndex        int
		audioDefaultStreamIndex int
		process                 string
	)

	var vFileInfo VideoFileInfoProbe
	err := json.Unmarshal([]byte(fileData), &vFileInfo)
	if err != nil {
		return err
	}

	subStreamIndex := 0
	for _, s := range vFileInfo.Streams {

		// Check that file codec is not h265. h265 transcoding to h264 is not supported yet :).
		if s.CodecType == "video" && s.CodecName == "h265" || s.CodecName == "hevc" {
			err := fmt.Sprintf("%v encoding is not supported", s.CodecName)
			return errors.New(err)
		}

		if s.CodecType == "audio" && s.CodecName == "aac" && s.Channels == 2 && s.Disposition.Default == 1 && strings.Contains(filepath.Ext(filePath), "mp4") {
			log.Println("File meets requirements. Remuxing not needed")
			return nil
		}

		// Extract subs
		if s.CodecType == "subtitle" {
			customNamingTag := ""
			if s.Tags.HandlerName == "Hearing Impaired" {
				customNamingTag = ".forced"
			}
			switch s.Tags.Language {
			case "eng", "en":
				extractSubs(s.Tags.Language, filePath, subStreamIndex, customNamingTag)
			case "spa", "es":
				if strings.Contains(s.Tags.Title, "Latin America") || strings.Contains(s.Tags.Title, "Latinoamérica") {
					extractSubs(s.Tags.Language, filePath, subStreamIndex, customNamingTag)
				}
			}

			subStreamIndex++
		}

		// Add to amount of audio streams
		if s.CodecType == "audio" {
			totalAudioStreams++
		}

		// Check if there is an audio AAC 2.0 stream that is not default
		if s.CodecType == "audio" && s.CodecName == "aac" && s.Channels == 2 {
			audioStreamIndex = totalAudioStreams - 1
			process = "disposition"
		} else if s.CodecType == "audio" && s.Disposition.Default == 1 {
			audioDefaultStreamIndex = totalAudioStreams - 1
		}

		if s.CodecType == "audio" && s.CodecName == "aac" && s.Channels != 2 && process == "" {
			process = "channelToStereo"
			audioStreamIndex = totalAudioStreams - 1
		}

		if s.CodecType == "audio" && s.CodecName != "aac" && process == "" {
			process = "encode"
			audioStreamIndex = totalAudioStreams - 1
		}

	}

	// Run needed ffmpeg commands
	if process == "disposition" {
		// Rename file to .original
		originalFile := fmt.Sprintf("%v.original", filePath)

		err = os.Rename(filePath, originalFile)
		if err != nil {
			return err
		}

		changeDefaultAudioStream(totalAudioStreams, audioStreamIndex, audioDefaultStreamIndex, originalFile, filePath)
	}
	if process == "channelToStereo" {
		// Rename file to .original
		originalFile := fmt.Sprintf("%v.original", filePath)

		err = os.Rename(filePath, originalFile)
		if err != nil {
			return err
		}

		createStereoAudioStream(totalAudioStreams, audioStreamIndex, originalFile, filePath)
	}
	if process == "encode" {
		// Rename file to .original
		originalFile := fmt.Sprintf("%v.original", filePath)

		err = os.Rename(filePath, originalFile)
		if err != nil {
			return err
		}

		encodeAudioStream(totalAudioStreams, audioStreamIndex, originalFile, filePath)
	}

	return nil
}

func extractSubs(language string, filePath string, subStreamIndex int, customNamingTag string) error {

	fileName := filepath.Base(filePath)
	fileDir := filepath.Dir(filePath)

	input := ffmpeg.Input(filePath, ffmpeg.KwArgs{"sub_charenc": "UTF-8"})

	subtitleIndex := fmt.Sprintf("s:%v", subStreamIndex)
	subtitle := input.Get(subtitleIndex)

	// Convert subtitles to vtt
	if language == "spa" {
		language = "es"
	}

	subtitleFileName := fmt.Sprintf("%v.%v%v.vtt", strings.TrimSuffix(fileName, filepath.Ext(fileName)), language, customNamingTag)
	fmt.Printf("Extracting Subtitle: %v", subtitleFileName)

	outputFileDir := fmt.Sprintf("%v/%v", fileDir, subtitleFileName)

	codecSubtitleStream := fmt.Sprintf("c:s:%v", subStreamIndex)
	out := ffmpeg.Output([]*ffmpeg.Stream{subtitle}, outputFileDir, ffmpeg.KwArgs{codecSubtitleStream: "webvtt"}).OverWriteOutput()

	out.Run()
	return nil
}

func changeDefaultAudioStream(totalAudioStreams, audioStreamIndex, audioDefaultStreamIndex int, originalFile, filePath string) error {
	log.Printf("Changing a:%v to default", audioStreamIndex)

	fileName := filepath.Base(filePath)
	fileDir := filepath.Dir(filePath)

	fileOutput := fmt.Sprintf("%v/%v.mp4", fileDir, strings.TrimSuffix(fileName, filepath.Ext(fileName)))

	input := ffmpeg.Input(originalFile)

	var streams []*ffmpeg.Stream

	streams = append(streams, input.Get("v:0")) // Append video stream to slice
	for i := 0; i <= totalAudioStreams-1; i++ {
		stream := fmt.Sprintf("a:%v", i)

		streams = append(streams, input.Get(stream))
	}

	newDefaultAudioStream := fmt.Sprintf("disposition:a:%v", audioStreamIndex)
	oldDefaultAudioStream := fmt.Sprintf("disposition:a:%v", audioDefaultStreamIndex)

	out := ffmpeg.Output(streams, fileOutput, ffmpeg.KwArgs{"c": "copy", newDefaultAudioStream: "default", oldDefaultAudioStream: 0, "movflags": "faststart"}).OverWriteOutput()
	out.Run()

	os.Remove(originalFile)
	return nil
}

func createStereoAudioStream(totalAudioStreams, audioStreamIndex int, originalFile, filePath string) error {
	log.Println("Creating AAC 2.0 audio stream.")

	fileName := filepath.Base(filePath)
	fileDir := filepath.Dir(filePath)

	fileOutput := fmt.Sprintf("%v/%v.mp4", fileDir, strings.TrimSuffix(fileName, filepath.Ext(fileName)))

	input := ffmpeg.Input(originalFile)

	var streams []*ffmpeg.Stream

	streams = append(streams, input.Get("v")) // Get video stream
	streams = append(streams, input.Get("a")) // Get all audio streams

	for i := 0; i <= totalAudioStreams-1; i++ {
		stream := fmt.Sprintf("a:%v", i)

		// only append the stream that is going to be downmixed to stereo and copied to a new audio stream
		if i == audioStreamIndex {
			streams = append(streams, input.Get(stream))
		}
	}

	addedAudioStream := fmt.Sprintf("c:a:%v", totalAudioStreams)

	out := ffmpeg.Output(streams, fileOutput, ffmpeg.KwArgs{"c:v": "copy", "ac": 2, addedAudioStream: "copy", "disposition:a": 0, "disposition:a:0": "default", "movflags": "faststart"}).OverWriteOutput()
	out.Run()

	os.Remove(originalFile)

	return nil
}

func encodeAudioStream(totalAudioStreams, audioStreamIndex int, originalFile, filePath string) error {
	log.Println("Converting and creating new AAC 2.0 audio stream.")

	fileName := filepath.Base(filePath)
	fileDir := filepath.Dir(filePath)

	fileOutput := fmt.Sprintf("%v/%v.mp4", fileDir, strings.TrimSuffix(fileName, filepath.Ext(fileName)))

	input := ffmpeg.Input(originalFile)

	var streams []*ffmpeg.Stream

	streams = append(streams, input.Get("v")) // Get video stream
	streams = append(streams, input.Get("a")) // Get all audio streams

	for i := 0; i <= totalAudioStreams-1; i++ {
		stream := fmt.Sprintf("a:%v", i)

		// only append the stream that is going to be copied
		if i == audioStreamIndex {
			streams = append(streams, input.Get(stream))
		}
	}

	addedAudioStream := fmt.Sprintf("c:a:%v", totalAudioStreams)

	out := ffmpeg.Output(streams, fileOutput, ffmpeg.KwArgs{"c:v": "copy", "c:a:0": "aac", "ac": 2, addedAudioStream: "copy", "disposition:a": 0, "disposition:a:0": "default", "movflags": "faststart"}).OverWriteOutput()
	out.Run()

	os.Remove(originalFile)

	return nil
}
