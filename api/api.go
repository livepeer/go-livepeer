package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

func CreateStream(apiKey string, transcodingOptions string) (string, error) {
	bearer := "Bearer " + apiKey
	f, err := ioutil.ReadFile(transcodingOptions)
	if err != nil {
		return "", err
	}
	body := bytes.NewBuffer(f)

	req, err := http.NewRequest("POST", "https://livepeer.com/api/stream", body)
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", bearer)
	req.Header.Add("content-type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	res, err := ioutil.ReadAll(resp.Body)
	if err != nil || string(res) == "" {
		return "", err
	}

	var stream map[string]interface{}
	err = json.Unmarshal([]byte(res), &stream)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%v", stream["id"]), nil
}

func GetBroadcaster(apiKey string) (string, error) {
	bearer := "Bearer " + apiKey
	req, err := http.NewRequest("GET", "https://livepeer.com/api/broadcaster", nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("Authorization", bearer)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	res, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var broadcasters []map[string]string
	err = json.Unmarshal([]byte(res), &broadcasters)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%v", broadcasters[0]["address"]), nil
}
