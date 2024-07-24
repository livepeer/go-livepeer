package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/golang/glog"
)

func (w *wizard) allTranscodingOptions() map[int]string {
	resp, err := http.Get(fmt.Sprintf("http://%v:%v/getAvailableTranscodingOptions", w.host, w.httpPort))
	if err != nil {
		glog.Errorf("Error getting all transcoding options: %v", err)
		return nil
	}

	defer resp.Body.Close()
	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Errorf("Error reading response: %v", err)
		return nil
	}

	var opts []string
	err = json.Unmarshal(result, &opts)
	if err != nil {
		glog.Errorf("Error unmarshalling all transcoding options: %v", err)
		return nil
	}
	sort.Strings(opts)

	optIds := make(map[int]string)

	wtr := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintln(wtr, "Identifier\tTranscoding Options")
	for idx, opt := range opts {
		fmt.Fprintf(wtr, "%v\t%v\n", idx, opt)
		optIds[idx] = opt
	}

	wtr.Flush()

	return optIds
}

func (w *wizard) setBroadcastConfig() {
	fmt.Printf("Enter a maximum transcoding price per pixel, in wei per pixels (pricePerUnit / pixelsPerUnit).\n")
	fmt.Printf("eg. 1 wei / 10 pixels = 0,1 wei per pixel \n")
	fmt.Printf("\n")
	fmt.Printf("Enter amount of pixels that make up a single unit (default: 1 pixel) - ")
	// Read numbers as strings not to lose precision and support big numbers
	pixelsPerUnit := w.readDefaultString("1")
	fmt.Printf("\n")
	fmt.Printf("Enter the currency for the price per unit (default: Wei) - ")
	currency := w.readDefaultString("Wei")
	fmt.Printf("\n")
	fmt.Printf("Enter the maximum price to pay for %s pixels in %s (default: 0) - ", pixelsPerUnit, currency)
	maxPricePerUnit := w.readDefaultString("0")

	opts := w.allTranscodingOptions()
	if opts == nil {
		return
	}

	fmt.Printf("Enter the identifiers of the video profiles you would like to use (use a comma as the delimiter in a list of identifiers) - ")
	idList := w.readString()

	transOpts, err := w.idListToVideoProfileList(idList, opts)
	if err != nil {
		glog.Error(err)
		return
	}

	val := url.Values{
		"pixelsPerUnit":      {fmt.Sprintf("%v", pixelsPerUnit)},
		"currency":           {fmt.Sprintf("%v", currency)},
		"maxPricePerUnit":    {fmt.Sprintf("%v", maxPricePerUnit)},
		"transcodingOptions": {fmt.Sprintf("%v", transOpts)},
	}

	result, ok := httpPostWithParams(fmt.Sprintf("http://%v:%v/setBroadcastConfig", w.host, w.httpPort), val)
	if !ok {
		fmt.Printf("Error applying configuration: %s\n", result)
	} else {
		fmt.Printf("Configuration applied successfully\n")
	}
}

func (w *wizard) idListToVideoProfileList(idList string, opts map[int]string) (string, error) {
	ids := strings.Split(idList, ",")

	var pListBuf bytes.Buffer
	for i, id := range ids {
		val, err := strconv.Atoi(strings.TrimSpace(id))
		if err != nil {
			return "", err
		}

		p, ok := opts[val]
		if !ok {
			return "", fmt.Errorf("not a valid identifier")
		} else {
			pListBuf.WriteString(p)

			if i < len(ids)-1 {
				pListBuf.WriteString(",")
			}
		}
	}

	return pListBuf.String(), nil
}
