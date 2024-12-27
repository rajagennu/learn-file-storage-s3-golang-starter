package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	fmt.Println(fileContentHeader)
	acceptedImageForamts := []string{"image/jpeg", "image/png"}
	validImageFormat := false
	for _, value := range acceptedImageForamts {
		if strings.Contains(fileContentHeader, value) {
			validImageFormat = true
			break
		}
	}
	if !validImageFormat {
		respondWithError(w, http.StatusBadRequest, "Expected image/jpeg, or image/png. Got "+fileContentHeader, err)
		return
	}

	imageExtention := strings.Split(fileContentHeader, "/")[1]
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
	assetPath := filepath.Join(cfg.assetsRoot, videoID.String()+"."+imageExtention)
	create, err := os.Create(assetPath)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to create file", err)
		return
	}
	defer create.Close()
	_, err = create.Write(imageInBytes)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error with copy", err)
		return
	}

	url := "http://localhost:" + os.Getenv("PORT") + "/assets/" + videoIDString + "." + imageExtention
	videoMetaData.ThumbnailURL = &url
	newVideoStruct := cfg.db.UpdateVideo(videoMetaData)
	respondWithJSON(w, http.StatusOK, newVideoStruct)
}
