package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	maxMemory := 1 << 30
	//r.Body = w.MaxBytesReader(w, r.Body, 1<<30)
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}
	fmt.Println("uploading video", videoID, "by user", userID)
	videoMetaData, _ := cfg.db.GetVideo(videoID)
	// TODO: implement the upload here
	err = r.ParseMultipartForm(int64(maxMemory))
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unable to parse using ParseMultipartForm", err)
		return
	}
	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}

	defer file.Close()
	fileContentHeader := header.Header.Get("Content-Type")
	fmt.Println(fileContentHeader)
	acceptedImageFormats := []string{"video/mp4"}
	validImageFormat := false
	for _, value := range acceptedImageFormats {
		if strings.Contains(fileContentHeader, value) {
			validImageFormat = true
			break
		}
	}
	if !validImageFormat {
		respondWithError(w, http.StatusBadRequest, "Expected video/mp4. Got "+fileContentHeader, err)
		return
	}

	f, err := os.CreateTemp("/tmp", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error while creating temporary file", err)
		return
	}
	defer os.Remove(f.Name()) // clean up

	_, err = io.Copy(f, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error while copying towards the temp file.", err)
		return
	}

	_, err = f.Seek(0, io.SeekStart)
	cryptRandVideoId := make([]byte, 32)
	_, err = rand.Read(cryptRandVideoId)
	aspectRatio, err := getVideoAspectRatio(f.Name())
	videoType := getMode(aspectRatio)
	fmt.Println(err)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error while calculating aspect ratio", err)
		return
	}
	fmt.Println("aspect ratio", aspectRatio)

	randomFileName := videoType + "/" + base64.RawURLEncoding.EncodeToString(cryptRandVideoId)
	processingFileName, err := processVideoForFastStart(f.Name())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error while generating moov version of the file ", err)
		return
	}
	processingFile, err := os.Open(processingFileName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error while opening the moov version of the file", err)
		return
	}

	_, err = cfg.s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(os.Getenv("S3_BUCKET")),

		Key:         aws.String(randomFileName + ".mp4"),
		Body:        processingFile,
		ContentType: aws.String(fileContentHeader),
	})
	defer processingFile.Close()

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error while uploading video", err)
		return
	}
	if err := f.Close(); err != nil {
		respondWithError(w, http.StatusBadRequest, "error while closing the temp file.", err)
		return
	}
	videoURL := fmt.Sprintf("https://%s/%s", cfg.s3CfDistribution, randomFileName+".mp4")
	//https://d3mfdc4yt332y0.cloudfront.net/portrait/DrCZCGszmLeC_BdDX6d9oPhjnFmQFud-hsRmYg6Az1A.mp4
	videoMetaData.VideoURL = &videoURL
	err = cfg.db.UpdateVideo(videoMetaData)
	//preSignedURL, err := cfg.dbVideoToSignedVideo(videoMetaData)
	//if err != nil {
	//	respondWithError(w, http.StatusInternalServerError, "error while uploading video", err)
	//	return
	//}

	respondWithJSON(w, http.StatusOK, videoMetaData)
}

func getMode(aspectRatio string) string {
	width := strings.Split(aspectRatio, ":")[0]
	height := strings.Split(aspectRatio, ":")[1]
	widthInt, _ := strconv.Atoi(width)
	heighInt, _ := strconv.Atoi(height)
	if widthInt > heighInt {
		return "landscape"
	}
	return "portrait"

}

func processVideoForFastStart(filePath string) (string, error) {
	newFilePath := filePath + ".processing"
	cmd := exec.Command("/usr/bin/ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", newFilePath)
	var result bytes.Buffer
	cmd.Stdout = &result
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return newFilePath, nil
}

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	preSignOption := s3.NewPresignClient(s3Client)
	preSignedURL, err := preSignOption.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	},
		s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", err
	}
	return preSignedURL.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	if video.VideoURL == nil {
		return video, nil
	}
	parts := strings.Split(*video.VideoURL, ",")
	if len(parts) < 2 {
		return video, nil
	}
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
