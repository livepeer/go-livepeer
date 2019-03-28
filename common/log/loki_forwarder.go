// Package log contains logging utilites.
// Right now contains only Loki forwarder.
package log

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	lokiclient "github.com/livepeer/loki-client/client"
	"github.com/livepeer/loki-client/model"
)

const glogTimeFormat = "20060102 15:04:05.999999"

var (
	levels       = []string{"info", "warning", "error", "fatal"}
	errShortLine = errors.New("Too short line")
	year         string
)

// ForwardLogsToLoki forwards data written to `os.Stderr` into `Loki` instance
func ForwardLogsToLoki(lokiURL, instanceName string) {
	glog.Infof("Forwarding logs from stderr to Loki instance %s with label instance=%s", lokiURL, instanceName)
	year = strconv.FormatInt(int64(time.Now().Year()), 10)
	started := make(chan interface{})
	go forwardLogsToLoki(started, lokiURL, instanceName)
	<-started
}

func logger(v ...interface{}) {
	fmt.Println(v...)
}

func forwardLogsToLoki(started chan interface{}, lokiURL, instanceName string) {
	pr, pw, err := os.Pipe()
	if err != nil {
		glog.Fatal(err)
	}
	baseLabels := model.LabelSet{"instance": instanceName}
	client, err := lokiclient.NewWithDefaults(lokiURL, baseLabels, logger)
	if err != nil {
		glog.Fatal(err)
	}

	realStderr := os.Stderr
	os.Stderr = pw
	tr := io.TeeReader(pr, realStderr)
	wg := sync.WaitGroup{}
	wg.Add(1)
	defer func() {
		pw.Close()
		os.Stderr = realStderr
		glog.Warning("Loki forwarding stopped")
	}()
	go func() {
		br := bufio.NewReader(tr)
		lastLineTime := time.Now()
		for {
			line, err := br.ReadString('\n')
			line = strings.TrimSpace(line)
			if len(line) > 0 {
				ts, level, file, caller, message, err := parseLine(line)
				if err != nil {
					client.Handle(nil, lastLineTime, line)
				} else if len(message) > 0 {
					lastLineTime = ts
					labels := model.LabelSet{"level": level, "file": file}
					f := "level=" + level + " caller=" + caller + ` msg="` + strings.Replace(message, `"`, `\"`, -1) + `"`
					client.Handle(labels, ts, f)
				}
			}
			if err != nil {
				break
			}
		}
		wg.Done()
	}()
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	go waitExit(client, c)
	close(started)
	wg.Wait()
}

func waitExit(client *lokiclient.Client, c chan os.Signal) {
	<-c
	client.Stop()
}

func parseLine(line string) (time.Time, string, string, string, string, error) {
	var t time.Time
	var level = "unknown"
	var file, caller, message string
	if len(line) < 31 {
		return t, level, file, caller, message, errShortLine
	}
	switch line[0] {
	case 'I':
		level = levels[0]
	case 'W':
		level = levels[1]
	case 'E':
		level = levels[2]
	case 'F':
		level = levels[3]
	}
	t, err := time.ParseInLocation(glogTimeFormat, year+line[1:21], time.Local)
	if err != nil {
		return t, level, file, caller, message, err
	}
	ll := line[30:]
	file = ll[:strings.IndexByte(ll, ':')]
	bi := strings.IndexByte(ll, ']')
	if bi+3 >= len(ll) {
		return t, level, file, caller, message, errShortLine
	}
	caller = ll[:bi]
	message = ll[bi+2:]
	if message[len(message)-1] == '\n' {
		message = message[:len(message)-1]
	}

	return t, level, file, caller, message, nil
}
