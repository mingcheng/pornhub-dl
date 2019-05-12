package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
)

// Global options
var debugMode bool

// Video stores informations about a single video on the platform.
type Video struct {
	url, title string
	qualities  map[string]VideoQuality
}

// VideoQuality stores informations about a single file of a specific video.
type VideoQuality struct {
	quality, url, filename string
}

func main() {
	// Print credits
	fmt.Println("| --- PornHub Downloader created by festie ---")
	fmt.Println("| GitHub: https://github.com/festie/pornhub-dl")
	fmt.Println("| --------------------------------------------")
	fmt.Println()

	// Define flags and parse them
	urlPtr := flag.String("url", "empty", "URL of the video to download")
	qualityPtr := flag.String("quality", "highest", "The quality number (eg. 720) or 'highest'")
	outputPtr := flag.String("output", "default", "Path to where the download should be saved or 'default' for the original filename")
	debugPtr := flag.Bool("debug", false, "Whether you want to activate debug mode or not")
	flag.Parse()

	// Assign variables to flag values
	url := *urlPtr
	quality := *qualityPtr
	outputPath := *outputPtr
	debugMode = *debugPtr

	// Check if parameters are set
	if url == "empty" {
		fmt.Println("Please pass a valid url with the -url parameter.")
		return
	}

	// Retrieve video details
	videoDetails, err := GetVideoDetails(url)
	if err != nil {
		fmt.Println("An error occoured while retrieving video details:")
		fmt.Println(err)
		return
	}

	// Print video details
	fmt.Println("Title: " + videoDetails.title)
	fmt.Println("Available formats:")
	for _, quality := range videoDetails.qualities {
		fmt.Printf("- %s (%s)\n", quality.quality, quality.filename)
	}

	// Process given quality
	var highestQuality int64
	selectedQualityName := quality
	if quality == "highest" {
		for i := range videoDetails.qualities {
			currentQuality, _ := strconv.ParseInt(i, 10, 64)
			if currentQuality > highestQuality {
				highestQuality = currentQuality
			}
		}

		selectedQualityName = strconv.FormatInt(highestQuality, 10)
	}

	if _, ok := videoDetails.qualities[selectedQualityName]; !ok {
		fmt.Println("Quality " + selectedQualityName + " is not available for this video.")
		return
	}

	selectedQuality := videoDetails.qualities[selectedQualityName]

	fmt.Println()
	fmt.Println("Selected format: " + selectedQuality.filename)

	if outputPath == "default" {
		outputPath = selectedQuality.filename
	}

	DownloadFile(outputPath, selectedQuality.url)
	fmt.Println("Done!")
}

// GetVideoDetails queries the given URL and returns details such as
// - title
// - available qualities
func GetVideoDetails(url string) (Video, error) {
	// Prepare regex-rules
	slashRegexRule := "\\\\/"
	videoRegexRule := "https://[a-zA-Z]{2}.phncdn.com/videos/[0-9]{6}/[0-9]{2}/[0-9]+/(([0-9]{3,4})P_[0-9]{3,4}K_[0-9]+.mp4)\\?[a-zA-Z0-9%=&_-]{0,210}"
	titleRegexRule := "<title>(.*) - Pornhub.com</title>"

	// Compile regex rules
	slashRegex, _ := regexp.Compile(slashRegexRule)
	videoRegex, _ := regexp.Compile(videoRegexRule)
	titleRegex, _ := regexp.Compile(titleRegexRule)

	// Download content of webpage
	resp, err := http.Get(url)

	// Check if there was an error downloading
	if err != nil {
		return Video{}, err
	}

	// Get body and manipulate it to make searching with regex easiert
	defer resp.Body.Close()
	source, err := ioutil.ReadAll(resp.Body)
	body := slashRegex.ReplaceAllString(string(source), "/")

	// Get title
	title := titleRegex.FindStringSubmatch(body)[1]

	// Get all videos from the website
	videoUrls := videoRegex.FindAllStringSubmatch(body, -1)
	videoQualities := make(map[string]VideoQuality)

	// Iterate through all found URLs to process them
	for _, slice := range videoUrls {
		// Continue if the result does not contain 3 items (url, filename, quality)
		if len(slice) != 3 {
			continue
		}

		// Get data
		url := slice[0]
		filename := slice[1]
		quality := slice[2]

		// Create and store video quality object
		videoQuality := VideoQuality{url: url, filename: filename, quality: quality}
		videoQualities[quality] = videoQuality
	}

	// Check if there are videos on the website. If not, cancel
	if len(videoQualities) == 0 {
		f, _ := os.Create("site.html")
		f.WriteString(body)
		return Video{}, errors.New("could not find any video sources")
	}

	// Sort qualities descending
	var qualityKeys []string
	for k := range videoQualities {
		qualityKeys = append(qualityKeys, k)
	}
	sort.Strings(qualityKeys)

	sortedQualities := make(map[string]VideoQuality)
	for _, k := range qualityKeys {
		sortedQualities[k] = videoQualities[k]
	}

	// Return the video detail instance
	video := Video{url: url, title: title, qualities: videoQualities}
	return video, nil
}

// DownloadStatus counts the written bytes. Because it implements the io.Writer interface,
// it can be given to the io.TeeReader(). This is also used to print out the current
// status of the download
type DownloadStatus struct {
	Done  uint64
	Total uint64
}

// Write implements io.Write
func (status *DownloadStatus) Write(bytes []byte) (int, error) {
	// Count the amount of bytes written since the last cycle
	byteAmount := len(bytes)

	// Increment current count by the amount of bytes written since the last cycle
	status.Done += uint64(byteAmount)

	// Update progress
	status.PrintDownloadStatus()

	// Return byteAmount
	return byteAmount, nil
}

// DownloadFile downloads a remote file to the harddrive while writing it
// directly to a file instead of storing it in RAM until the donwload completes.
func DownloadFile(filepath string, url string) error {

	// Create a temporary file
	tempfile := filepath + ".tmp"
	output, err := os.Create(tempfile)
	if err != nil {
		return err
	}
	defer output.Close()

	// Download data from given URL
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read total size of file
	filesize, _ := strconv.ParseUint(resp.Header.Get("Content-Length"), 10, 64)

	// Create our progress reporter and pass it to be used alongside our writer
	counter := &DownloadStatus{Total: filesize}
	_, err = io.Copy(output, io.TeeReader(resp.Body, counter))
	if err != nil {
		return err
	}

	// Rename temp file to correct ending
	err = os.Rename(tempfile, filepath)
	if err != nil {
		return err
	}

	return nil
}

// PrintDownloadStatus prints the current download progress to console.
func (status DownloadStatus) PrintDownloadStatus() {
	// Clear line
	fmt.Printf("\r%s", strings.Repeat(" ", 35))

	// Print current status
	fmt.Printf("\rDownloading (%s / %s)... ", humanize.Bytes(status.Done), humanize.Bytes(status.Total))
}
