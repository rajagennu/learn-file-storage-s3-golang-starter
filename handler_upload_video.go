package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
	"io"
	"net/http"
	"os"
	"strings"
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

	f, err := os.CreateTemp("", "tubely-upload.mp4")
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
	randomFileName := base64.RawURLEncoding.EncodeToString(cryptRandVideoId)
	_, err = cfg.s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(os.Getenv("S3_BUCKET")),

		Key:         aws.String(randomFileName + ".mp4"),
		Body:        f,
		ContentType: aws.String(fileContentHeader),
	})
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error while uploading video", err)
		return
	}
	if err := f.Close(); err != nil {
		respondWithError(w, http.StatusBadRequest, "error while closing the temp file.", err)
		return
	}
	videoURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", os.Getenv("S3_BUCKET"), os.Getenv("S3_REGION"), randomFileName+".mp4")
	videoMetaData.VideoURL = &videoURL
	newVideoStruct := cfg.db.UpdateVideo(videoMetaData)
	respondWithJSON(w, http.StatusOK, newVideoStruct)
}
