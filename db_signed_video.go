package main

import (
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"log"
	"strings"
	"time"
)

func (cfg *apiConfig) dbVideoToSignedVideo2(video database.Video) (database.Video, error) {
	log.Println("Converting video to signed video" + *video.VideoURL)
	bucket := strings.Split(*video.VideoURL, ",")[0]
	key := strings.Split(*video.VideoURL, ",")[1]
	preSignedURL, err := generatePresignedURL(cfg.s3Client, bucket, key, time.Minute*15)
	if err != nil {
		return database.Video{}, err
	}
	video.VideoURL = &preSignedURL
	return video, nil
}
