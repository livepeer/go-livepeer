package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/livepeer/go-livepeer/build"

	"github.com/ethereum/go-ethereum/log"
	lpcommon "github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/eth"
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

func (w *wizard) readMultilineString() string {
	fmt.Printf("(press enter followed by %s when done) > ", build.AcceptMultiline)

	var buf strings.Builder
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		buf.WriteString(fmt.Sprintln(scanner.Text()))
	}

	return strings.TrimSuffix(buf.String(), "\n")
}

// readStringAndValidate reads a single line from stdin, trims spaces and
// checks that the string passes a condition defined by the provided validation function
func (w *wizard) readStringAndValidate(validate func(in string) (string, error)) string {
	for {
		fmt.Printf("> ")
		text, err := w.in.ReadString('\n')
		if err != nil {
			log.Crit("Failed to read user input", "err", err)
		}
		text = strings.TrimSpace(text)
		validText, err := validate(text)
		if err != nil {
			log.Error("Failed to validate input", "err", err)
			continue
		}
		return validText
	}
}

// readStringYesOrNo reads a single line from stdin, trims spaces and
// checks that the string is either y or n
func (w *wizard) readStringYesOrNo() string {
	return w.readStringAndValidate(func(in string) (string, error) {
		if in != "y" && in != "n" {
			return "", errors.New("Enter y or n")
		}

		return in, nil
	})
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

func (w *wizard) readBaseAmountAndValidate(validate func(in *big.Int) error) *big.Int {
	for {
		text := w.readString()
		val, err := eth.ToBaseAmount(text, eth.DefaultMaxDecimals)
		if err != nil {
			log.Error("Error parsing user input", "err", err)
			continue
		}
		if err := validate(val); err != nil {
			log.Error("Invalid user input", "err", err)
			continue
		}
		return val
	}
}

func (w *wizard) readPositiveBaseAmount() *big.Int {
	return w.readBaseAmountAndValidate(func(in *big.Int) error {
		if in.Cmp(big.NewInt(0)) < 0 {
			return errors.New("base amount must be positive")
		}

		return nil
	})
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

func (w *wizard) readBigInt(prompt string) *big.Int {
	for {
		fmt.Printf(fmt.Sprintf("%s - > ", prompt))
		text, err := w.in.ReadString('\n')
		if err != nil {
			log.Crit("Failed to read user input", "err", err)
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

func (w *wizard) readFloatAndValidate(validate func(in float64) (float64, error)) float64 {
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
		validFloat, err := validate(val)
		if err != nil {
			log.Error("Failed to validate input", "err", err)
			continue
		}
		return validFloat
	}
}

func (w *wizard) readPositiveFloat() float64 {
	return w.readFloatAndValidate(func(in float64) (float64, error) {
		if in < 0 {
			return 0, errors.New("value must be positive")
		}

		return in, nil
	})
}

func (w *wizard) readPositiveFloatAndValidate(validate func(in float64) (float64, error)) float64 {
	return w.readFloatAndValidate(func(in float64) (float64, error) {
		if in < 0 {
			return 0, errors.New("value must be positive")
		}
		if _, err := validate(in); err != nil {
			return 0, err
		}

		return in, nil
	})
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
		log.Error("Error sending HTTP GET", "url", url, "err", err)
		return ""
	}

	defer resp.Body.Close()
	result, err := ioutil.ReadAll(resp.Body)
	if err != nil || string(result) == "" {
		return ""
	}
	return string(result)

}

func httpPostWithParams(url string, val url.Values) (string, bool) {
	return httpPostWithParamsHeaders(url, val, map[string]string{})
}

func httpPostWithParamsHeaders(url string, val url.Values, headers map[string]string) (string, bool) {
	body := bytes.NewBufferString(val.Encode())
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		log.Error("Error creating HTTP POST", "url", url, "err", err)
		return "", false
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error("Error sending HTTP POST", "url", url, "err", err)
		return "", false
	}

	defer resp.Body.Close()
	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", false
	}

	return string(result), resp.StatusCode >= 200 && resp.StatusCode < 300
}

func httpPost(url string) string {
	resp, err := http.Post(url, "application/x-www-form-urlencoded", nil)
	if err != nil {
		log.Error("Error sending HTTP POST: ", "url", url, "err", err)
		return ""
	}

	defer resp.Body.Close()
	result, err := ioutil.ReadAll(resp.Body)
	if err != nil || string(result) == "" {
		return ""
	}

	return string(result)
}
