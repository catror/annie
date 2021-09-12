package ixigua

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/iawia002/annie/extractors/types"
	"github.com/iawia002/annie/request"
	"github.com/iawia002/annie/utils"
)

type ssrHydratedData struct {
	AnyVideo struct {
		GidInformation struct {
			PackerData struct {
				Video struct {
					Title         string `json:"title"`
					VideoResource struct {
						Normal struct {
							VideoList struct {
								Video1 video `json:"video_1"`
								Video2 video `json:"video_2"`
								Video3 video `json:"video_3"`
								Video4 video `json:"video_4"`
							} `json:"video_list"`
						} `json:"normal"`
						Dash120Fps struct {
							DynamicVideo struct {
								DynamicVideoList []video `json:"dynamic_video_list"`
								DynamicAudioList []video `json:"dynamic_audio_list"`
							} `json:"dynamic_video"`
						} `json:"dash_120fps"`
					} `json:"videoResource"`
				} `json:"video"`
			} `json:"packerData"`
		} `json:"gidInformation"`
	} `json:"anyVideo"`
}

type ssrHydratedDataEpisode struct {
	AnyVideo struct {
		GidInformation struct {
			PackerData struct {
				EpisodeInfo struct {
					Title string `json:"title"`
					Name  string `json:"name"`
				} `json:"episodeInfo"`
				VideoResource struct {
					Normal struct {
						VideoList struct {
							Video1 video `json:"video_1"`
							Video2 video `json:"video_2"`
							Video3 video `json:"video_3"`
							Video4 video `json:"video_4"`
						} `json:"video_list"`
					} `json:"normal"`
				} `json:"videoResource"`
			} `json:"packerData"`
		} `json:"gidInformation"`
	} `json:"anyVideo"`
}

type video struct {
	Definition string `json:"definition"`
	MainURL    string `json:"main_url"`
}

const (
	referer       = "https://www.ixigua.com"
	defaultCookie = "MONITOR_WEB_ID=7892c49b-296e-4499-8704-e47c1b150c18; ixigua-a-s=1; ttcid=af99669b6304453480454f150701d5c226; BD_REF=1; __ac_nonce=060d88ff000a75e8d17eb; __ac_signature=_02B4Z6wo00f01kX9ZpgAAIDAKIBBQUIPYT5F2WIAAPG2ad; ttwid=1%7CcIsVF_3vqSIk4XErhPB0H2VaTxT0tdsTMRbMjrJOPN8%7C1624806049%7C08ce7dd6f7d20506a41ba0a331ef96a6505d96731e6ad9f6c8c709f53f227ab1"
)

type extractor struct{}

// New returns a ixigua extractor.
func New() types.Extractor {
	return &extractor{}
}

// Extract is the main function to extract the data.
func (e *extractor) Extract(url string, option types.Options) ([]*types.Data, error) {
	html, err := request.Get(url, referer, map[string]string{
		"cookie":     getCookie(option.Cookie),
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/79.0.3945.88 Safari/537.36",
	})
	if err != nil {
		return nil, err
	}
	jsonRegexp := utils.MatchOneOf(html, `window\._SSR_HYDRATED_DATA=(.*?)</script>`)
	if jsonRegexp == nil || len(jsonRegexp) < 2 {
		return nil, types.ErrURLParseFailed
	}
	jsonStr := strings.ReplaceAll(string(jsonRegexp[1]), ":undefined", ":\"undefined\"")
	var (
		title   string
		streams map[string]*types.Stream
	)
	if regexp.MustCompile(`"albumId"`).MatchString(html) {
		var ratedData ssrHydratedDataEpisode
		if err := json.Unmarshal([]byte(jsonStr), &ratedData); err != nil {
			return nil, err
		}
		episodeInfo := ratedData.AnyVideo.GidInformation.PackerData.EpisodeInfo
		title = fmt.Sprintf("%s %s", episodeInfo.Title, episodeInfo.Name)
		if streams, err = getStreamsEpisode(&ratedData); err != nil {
			return nil, err
		}
	} else {
		var ratedData ssrHydratedData
		if err := json.Unmarshal([]byte(jsonStr), &ratedData); err != nil {
			return nil, err
		}
		title = ratedData.AnyVideo.GidInformation.PackerData.Video.Title
		if streams, err = getStreams(&ratedData); err != nil {
			return nil, err
		}
	}
	return []*types.Data{
		{
			Site:    "西瓜视频 ixigua.com",
			Title:   title,
			Type:    types.DataTypeVideo,
			Streams: streams,
			URL:     url,
		},
	}, nil

}

func getCookie(c string) string {
	if c != "" {
		return c
	}
	return defaultCookie
}

func getStreams(d *ssrHydratedData) (map[string]*types.Stream, error) {
	streams := make(map[string]*types.Stream)
	videoList := d.AnyVideo.GidInformation.PackerData.Video.VideoResource.Dash120Fps.DynamicVideo.DynamicVideoList
	audioList := d.AnyVideo.GidInformation.PackerData.Video.VideoResource.Dash120Fps.DynamicVideo.DynamicAudioList
	audioCount := len(audioList)
	if audioCount > 0 {
		audioURL := base64Decode(audioList[audioCount-1].MainURL)
		audioSize, err := request.Size(audioURL, referer)
		audioPart := &types.Part{
			URL:  audioURL,
			Size: audioSize,
			Ext:  "mp3",
		}
		if err != nil {
			return nil, err
		}
		for _, i := range videoList {
			if i.MainURL == "" {
				continue
			}
			videoURL := base64Decode(i.MainURL)
			videoSize, err := request.Size(videoURL, referer)
			if err != nil {
				return nil, err
			}
			videoPart := &types.Part{
				URL:  videoURL,
				Size: videoSize,
				Ext:  "mp4",
			}
			streams[i.Definition] = &types.Stream{
				ID:      i.Definition,
				Quality: i.Definition,
				Parts:   []*types.Part{videoPart, audioPart},
				Size:    audioSize + videoSize,
				Ext:     "mp4",
				NeedMux: true,
			}

		}
		return streams, nil
	}
	Normal := d.AnyVideo.GidInformation.PackerData.Video.VideoResource.Normal.VideoList
	NormalList := []video{Normal.Video1, Normal.Video2, Normal.Video3, Normal.Video4}
	for _, i := range NormalList {
		if i.MainURL == "" {
			continue
		}
		videoURL := base64Decode(i.MainURL)
		videoSize, err := request.Size(videoURL, referer)
		if err != nil {
			return nil, err
		}
		videoPart := &types.Part{
			URL:  videoURL,
			Size: videoSize,
			Ext:  "mp4",
		}
		streams[i.Definition] = &types.Stream{
			ID:      i.Definition,
			Quality: i.Definition,
			Parts:   []*types.Part{videoPart},
			Size:    videoSize,
			Ext:     "mp4",
		}
	}
	return streams, nil
}

func getStreamsEpisode(d *ssrHydratedDataEpisode) (map[string]*types.Stream, error) {
	streams := make(map[string]*types.Stream)
	Normal := d.AnyVideo.GidInformation.PackerData.VideoResource.Normal.VideoList
	NormalList := []video{Normal.Video1, Normal.Video2, Normal.Video3, Normal.Video4}
	for _, i := range NormalList {
		if i.MainURL == "" {
			continue
		}
		videoURL := base64Decode(i.MainURL)
		videoSize, err := request.Size(videoURL, referer)
		if err != nil {
			return nil, err
		}
		videoPart := &types.Part{
			URL:  videoURL,
			Size: videoSize,
			Ext:  "mp4",
		}
		streams[i.Definition] = &types.Stream{
			ID:      i.Definition,
			Quality: i.Definition,
			Parts:   []*types.Part{videoPart},
			Size:    videoSize,
		}
	}
	return streams, nil
}

func base64Decode(t string) string {
	d, _ := base64.StdEncoding.DecodeString(t)
	return string(d)
}
