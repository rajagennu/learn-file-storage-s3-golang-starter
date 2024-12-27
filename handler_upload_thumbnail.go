package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 10 << 20

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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unable to parse using ParseMultipartForm", err)
		return
	}
	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}

	defer file.Close()
	fileContentHeader := header.Header.Get("Content-Type")
	imageInBytes, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to read form file", err)
		return
	}
	videoMetaData, _ := cfg.db.GetVideo(videoID)
	newThumbnail := thumbnail{data: imageInBytes, mediaType: fileContentHeader}
	videoThumbnails[videoID] = newThumbnail

	//fmt.Println(fileContentHeader, imageInBytes)
	//url := "localhost:" + os.Getenv("PORT") + "/app/thumbnails/" + videoIDString
	//videoMetaData.ThumbnailURL = &url
	encodedString := base64.StdEncoding.EncodeToString(imageInBytes)
	dataURL := "data:" + fileContentHeader + ";base64," + encodedString
	videoMetaData.ThumbnailURL = &dataURL
	newVideoStruct := cfg.db.UpdateVideo(videoMetaData)
	respondWithJSON(w, http.StatusOK, newVideoStruct)

}
