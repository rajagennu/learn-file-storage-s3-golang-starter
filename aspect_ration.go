package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
)

func getVideoAspectRatio(filePath string) (string, error) {
	fmt.Println("Filepath " + filePath)
	fmt.Println("Filepath " + filePath)
	if filePath == "" {
		return "", errors.New("filePath is empty")
	}
	var streamedResult map[string]any
	cmd := exec.Command("/usr/bin/ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var result bytes.Buffer
	cmd.Stdout = &result
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(result.Bytes(), &streamedResult)
	if err != nil {
		return "", err
	}
	//fmt.Println(streamedResult)
	streams, ok := streamedResult["streams"].([]any)
	if !ok {
		fmt.Println("streams is not an array")
		return "", nil
	}
	type dimentions struct {
		width  int
		height int
	}
	newDmintion := dimentions{}
	for _, stream := range streams {
		data, ok := stream.(map[string]any)
		if !ok {
			continue
		}
		if width, exists := data["width"]; exists {
			newDmintion.width = int(width.(float64))
		}
		if height, exists := data["height"]; exists {
			newDmintion.height = int(height.(float64))
		}
	}
	var gcd func(h, w int) int
	gcd = func(h, w int) int {
		if w == 0 {
			return h
		}
		return gcd(w, h%w)
	}
	fmt.Println(newDmintion)
	gcdValue := gcd(newDmintion.width, newDmintion.height)
	aspectRatio := fmt.Sprintf("%d:%d", newDmintion.width/gcdValue, newDmintion.height/gcdValue)
	return aspectRatio, nil
}
