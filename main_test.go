package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const url = "https://www.pornhub.com/view_video.php?viewkey=ph5ea6eece5ab3e"

func TestGetVideoDetails(t *testing.T) {
	got, err := GetVideoDetails(url)

	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.NotEmpty(t, got.title)
	assert.NotEmpty(t, got.qualities)

	for _, v := range got.qualities {
		fmt.Printf("%s(%d) - %s\n", v.quality, v.filesize, v.url)
	}
}

func TestDownloadFile(t *testing.T) {
	got, err := GetVideoDetails(url)
	assert.NoError(t, err)

	output, err := ioutil.TempFile(os.TempDir(), "1.mp4")
	assert.NoError(t, err)

	defer os.Remove(output.Name())

	for _, v := range got.qualities {
		err := DownloadFile(output.Name(), v.url)
		assert.NoError(t, err)
		return
	}
}

func TestSplitDownloadFile(t *testing.T) {
	got, err := GetVideoDetails(url)
	assert.NoError(t, err)

	output, err := ioutil.TempFile(os.TempDir(), "1.mp4")
	assert.NoError(t, err)
	defer os.Remove(output.Name())

	for _, v := range got.qualities {
		err := SplitDownloadFile(output.Name(), v)
		assert.NoError(t, err)
		return
	}
}
