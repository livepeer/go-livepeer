package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/log"
	lpcommon "github.com/livepeer/go-livepeer/common"
)

// // read reads a single line from stdin, trimming if from spaces.
func (w *wizard) read() string {
	fmt.Printf("> ")
	text, err := w.in.ReadString('\n')
	if err != nil {
		log.Crit("Failed to read user input", "err", err)
	}
	return strings.TrimSpace(text)
}

// readString reads a single line from stdin, trimming if from spaces, enforcing
// non-emptyness.
func (w *wizard) readString() string {
	for {
		fmt.Printf("> ")
		text, err := w.in.ReadString('\n')
		if err != nil {
			log.Crit("Failed to read user input", "err", err)
		}
		if text = strings.TrimSpace(text); text != "" {
			return text
		}
	}
}

// readStringAndValidate reads a single line from stdin, trims spaces and
// checks that the string passes a condition defined by the provided validation function
func (w *wizard) readStringAndValidate(validate func(in string) error) string {
	for {
		fmt.Printf("> ")
		text, err := w.in.ReadString('\n')
		if err != nil {
			log.Crit("Failed to read user input", "err", err)
		}
		text = strings.TrimSpace(text)
		if err := validate(text); err != nil {
			log.Error("Failed to validate input", "err", err)
			continue
		}
		return text
	}
}

// readDefaultString reads a single line from stdin, trimming if from spaces. If
// an empty line is entered, the default value is returned.
func (w *wizard) readDefaultString(def string) string {
	fmt.Printf("> ")
	text, err := w.in.ReadString('\n')
	if err != nil {
		log.Crit("Failed to read user input", "err", err)
	}
	if text = strings.TrimSpace(text); text != "" {
		return text
	}
	return def
}

// readInt reads a single line from stdin, trimming if from spaces, enforcing it
// to parse into an integer.
func (w *wizard) readInt() int {
	for {
		fmt.Printf("> ")
		text, err := w.in.ReadString('\n')
		if err != nil {
			log.Crit("Failed to read user input", "err", err)
		}
		if text = strings.TrimSpace(text); text == "" {
			continue
		}
		val, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			log.Error("Invalid input, expected integer", "err", err)
			continue
		}
		return val
	}
}

func (w *wizard) readDefaultInt(def int) int {
	fmt.Printf("> ")
	text, err := w.in.ReadString('\n')
	if err != nil {
		log.Crit("Failed to read user input", "err", err)
	}
	val, err := strconv.Atoi(strings.TrimSpace(text))
	if err == nil {
		return val
	}
	return def
}

func (w *wizard) readBigInt() *big.Int {
	for {
		fmt.Printf("> ")
		text, err := w.in.ReadString('\n')
		if err != nil {
			log.Crit("Failed to read user input", "err", err)
		}
		if text = strings.TrimSpace(text); text == "" {
			continue
		}
		val, err := lpcommon.ParseBigInt(strings.TrimSpace(text))
		if err != nil {
			log.Error("Invalid input, expected big integer", "err", err)
			continue
		}
		return val
	}
}

func (w *wizard) readDefaultBigInt(def *big.Int) *big.Int {
	fmt.Printf("> ")
	text, err := w.in.ReadString('\n')
	if err != nil {
		log.Crit("Failed to read user input", "err", err)
	}
	val, err := lpcommon.ParseBigInt(strings.TrimSpace(text))
	if err == nil {
		return val
	}
	return def
}

func (w *wizard) readFloat() float64 {
	for {
		fmt.Printf("> ")
		text, err := w.in.ReadString('\n')
		if err != nil {
			log.Crit("Failed to read user input", "err", err)
		}
		if text = strings.TrimSpace(text); text == "" {
			continue
		}
		val, err := strconv.ParseFloat(text, 64)
		if err != nil {
			log.Error("Invalid input, expected float", "err", err)
			continue
		}
		return val
	}
}

func (w *wizard) readDefaultFloat(def float64) float64 {
	fmt.Printf("> ")
	text, err := w.in.ReadString('\n')
	if err != nil {
		log.Crit("Failed to read user input", "err", err)
	}
	val, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err == nil {
		return val
	}
	return def
}

func httpGet(url string) string {
	resp, err := http.Get(url)
	if err != nil {
		log.Error("Error sending HTTP GET: %v")
		return ""
	}

	defer resp.Body.Close()
	result, err := ioutil.ReadAll(resp.Body)
	if err != nil || string(result) == "" {
		return ""
	}
	return string(result)

}

func httpPostWithParams(url string, val url.Values) string {
	body := bytes.NewBufferString(val.Encode())

	resp, err := http.Post(url, "application/x-www-form-urlencoded", body)
	if err != nil {
		log.Error("Error sending HTTP POST: %v", err)
		return ""
	}

	defer resp.Body.Close()
	result, err := ioutil.ReadAll(resp.Body)
	if err != nil || string(result) == "" {
		return ""
	}

	return string(result)
}

func httpPost(url string) string {
	resp, err := http.Post(url, "application/x-www-form-urlencoded", nil)
	if err != nil {
		log.Error("Error sending HTTP POST: %v", err)
		return ""
	}

	defer resp.Body.Close()
	result, err := ioutil.ReadAll(resp.Body)
	if err != nil || string(result) == "" {
		return ""
	}

	return string(result)
}
