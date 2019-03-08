package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

type authWebhookReq struct {
	url string `json:"url"`
}

func main() {
	http.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		out, _ := ioutil.ReadAll(r.Body)
		fmt.Printf("RTMP URL: %v", string(out))

		var req authWebhookReq
		err := json.Unmarshal(out, &req)
		if err != nil {
			fmt.Printf("Error parsing URL: %v\n", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		u, err := url.Parse(req.url)
		m, _ := url.ParseQuery(u.RawQuery)
		mid := m.Get("ManifestID")

		if mid != "" {
			fmt.Printf("Using %v as ManifestID", mid)
			w.Write([]byte(fmt.Sprintf("{\"ManifestID\":\"%v\"}", mid)))
		} else {
			fmt.Printf("Using \"abc\" as ManifestID")
			w.Write([]byte(`{"ManifestID":"abc"}`))
		}
	})

	err := http.ListenAndServe(":8000", nil) // set listen port
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
