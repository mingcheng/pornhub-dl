package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"text/tabwriter"

	"github.com/dustin/go-humanize"
	goproxy "golang.org/x/net/proxy"
)

// Global options
var debugMode bool
var threads = 10
var socks5 = ""

const userAgent = `Mozilla/5.0 (Windows NT 6.3; Trident/7.0; rv:11.0) like Gecko`

// States
var counter *DownloadStatus
var wg sync.WaitGroup
var waitingThreads = 0

// Video stores informations about a single video on the platform.
type Video struct {
	url, title string
	qualities  map[string]VideoQuality
}

// VideoQuality stores informations about a single file of a specific video.
type VideoQuality struct {
	quality, url, filename string
	filesize               uint64
	ranges                 bool
}

func main() {
	// Print credits
	fmt.Println()
	fmt.Println("| --- PornHub Downloader created by mingcheng(based on festie's code) ---")
	fmt.Println("| GitHub: https://github.com/festie/pornhub-dl https://github.com/mingcheng/pornhub-dl")
	fmt.Println("| --------------------------------------------")
	fmt.Println()

	// Define flags and parse them
	urlPtr := flag.String("url", "empty", "URL of the video to download")
	qualityPtr := flag.String("quality", "highest", "The quality number (eg. 720) or 'highest'")
	outputPtr := flag.String("output", "default", "Path to where the download should be saved or 'default' for the original filename")
	threadsPtr := flag.Int("threads", 5, "The amount of threads to use to download")
	flag.BoolVar(&debugMode, "debug", false, "Whether you want to activate debug mode or not")
	flag.StringVar(&socks5, "socks5", "", "Specify socks5 proxy address for downloading resources")
	flag.Parse()

	// Assign variables to flag values
	url := *urlPtr
	quality := *qualityPtr
	outputPath := *outputPtr
	threads = *threadsPtr

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

	// Print video details
	fmt.Println("Title: " + videoDetails.title + "\n")
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 6, 8, 2, ' ', 0)

	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t\n", "Quality", "Filename", "Size", "FastDL", "")
	// fmt.Fprintf(w, "\n%s\t%s\t%s\t%s\t%s\t", "-------", "--------", "----", "------", "")

	for _, quality := range videoDetails.qualities {
		x := " "
		if selectedQualityName == quality.quality {
			x = "←"
		}

		fmt.Fprintf(w, "%sp\t%s\t%s\t%t\t%s\t\n", quality.quality, quality.filename, humanize.Bytes(quality.filesize), quality.ranges, x)
	}

	w.Flush()

	if _, ok := videoDetails.qualities[selectedQualityName]; !ok {
		fmt.Println("Quality " + selectedQualityName + " is not available for this video.")
		return
	}

	selectedQuality := videoDetails.qualities[selectedQualityName]

	fmt.Println("")

	if outputPath == "default" {
		outputPath = selectedQuality.filename
	}

	if selectedQuality.ranges {
		SplitDownloadFile(outputPath, selectedQuality)
	} else {
		DownloadFile(outputPath, selectedQuality.url)
	}
}

// Start HTTP GET Request
// - url
// - proxyAddr
func getResp(url string) (*http.Response, error) {
	httpTransport := &http.Transport{}
	client := &http.Client{Transport: httpTransport}

	if len(socks5) > 0 {
		_, _ = fmt.Fprintf(os.Stderr, "Socks5 proxy address is %s\n", socks5)
		dialer, err := goproxy.SOCKS5("tcp", socks5, nil, goproxy.Direct)
		if err != nil {
			return nil, err
		}

		httpTransport.DialContext = func(ctx context.Context, network, addr string) (conn net.Conn, e error) {
			return dialer.Dial(network, addr)
		}
	}

	request, err := http.NewRequest("GET", url, nil)
	request.Header.Add("User-Agent", userAgent)
	request.Header.Add("Referer", url)
	if err != nil {
		return nil, err
	}

	return client.Do(request)
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
	// resp, err := http.Get(url)
	resp, err := getResp(url)

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
		ranges := false
		var filesize uint64

		// Do Head-Request to the file to fetch some data
		headResp, err := http.Head(url)
		if err != nil {
			fmt.Println("Error while preparing download:")
			fmt.Println(err)
		}
		defer headResp.Body.Close()

		// Get size of file
		filesize, _ = strconv.ParseUint(headResp.Header.Get("Content-Length"), 10, 64)

		// Check if the server supports the Range-header
		if headResp.Header.Get("Accept-Ranges") == "bytes" {
			ranges = true
		}

		// Create and store video quality object
		videoQuality := VideoQuality{url: url, filename: filename, quality: quality, filesize: filesize, ranges: ranges}
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
	Done, Total uint64
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

// SplitDownloadFile downloads a remote file to the harddrive while writing it
// directly to a file instead of storing it in RAM until the donwload completes.
func SplitDownloadFile(filepath string, video VideoQuality) error {
	counter = &DownloadStatus{Total: video.filesize}
	sliceSize := video.filesize / uint64(threads)

	for i := 1; i <= threads; i++ {
		offset := sliceSize * uint64(i-1)
		end := offset + sliceSize - 1

		if i == threads {
			end = video.filesize
		}

		// Create a temporary file
		tempfileName := fmt.Sprintf("%s.%d.tmp", filepath, i)
		output, err := os.Create(tempfileName)
		if err != nil {
			return err
		}

		wg.Add(1)
		go DoPartialDownload(video.url, offset, end, output)
	}

	fmt.Printf("Downloading file using %d threads.\n", threads)
	wg.Wait()
	counter.PrintDownloadStatus()
	fmt.Print("\nProcessing... ")

	// Combine single downloads into a single video file
	output, _ := os.Create(filepath)
	defer output.Close()
	for i := 1; i <= threads; i++ {
		// Open temporary file
		tempfileName := fmt.Sprintf("%s.%d.tmp", filepath, i)
		file, _ := os.Open(tempfileName)

		// Read bytes
		stat, _ := file.Stat()
		tempBytes := make([]byte, stat.Size())
		file.Read(tempBytes)

		// Write to output file
		output.Write(tempBytes)

		// Close and delete temporary file
		file.Close()

		err := os.Remove(tempfileName)
		if err != nil {
			fmt.Println(err)
		}
	}

	fmt.Println("Done!")
	fmt.Println()

	return nil
}

// DoPartialDownload downloads a special part of the file at the given URL.
func DoPartialDownload(url string, offset uint64, end uint64, output *os.File) ([]byte, error) {
	defer wg.Done()
	client := http.Client{}

	// Build request
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", offset, end))
	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("Referer", url)

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Create our progress reporter and pass it to be used alongside our writer
	_, err = io.Copy(output, io.TeeReader(bytes.NewReader(data), counter))
	if err != nil {
		return nil, err
	}

	output.Close()
	waitingThreads++

	return data, nil
}

// DownloadFile downloads a remote file to the harddrive while writing it
// directly to a file instead of storing it in RAM until the donwload completes.
func DownloadFile(filepath string, url string) error {
	fmt.Println("Server does not support partial downloads. Continuing with a single thread.")

	// Create a temporary file
	tempfile := filepath + ".tmp"
	output, err := os.Create(tempfile)
	if err != nil {
		return err
	}
	defer output.Close()

	// Download data from given URL
	// resp, err := http.Get(url)
	resp, err := getResp(url)
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
	// Print current status
	fmt.Printf("\rDownloading %s / %s (%d of %d threads done) ", humanize.Bytes(status.Done), humanize.Bytes(status.Total), waitingThreads, threads)
}
