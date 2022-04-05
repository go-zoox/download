package download

import "testing"

func TestDownload(t *testing.T) {
	url := "https://cdn-transcode.jingdaka.com/video/2020/09/12/c8067037-f083-4c63-bb80-b09ce9e5ae20.mp4"
	fileName := "test.mp4"
	err := Download(url, &Config{
		FilePath: fileName,
	})
	if err != nil {
		t.Error(err)
	}
}
