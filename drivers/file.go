package drivers

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/livepeer/go-livepeer/net"
)

type filesystem struct {
	workDir string
}
type filesystemSession struct {
	sessName string
	workDir  string
}

func NewFilesystemDriver(workDir string) OSDriver {
	return &filesystem{workDir}
}

func (fs *filesystem) SaveData(path string, data []byte) error {
	_, err := fs.NewSession("").SaveData(path, data)
	return err
}

func (fs *filesystem) GetData(path string) ([]byte, error) {
	fname := filepath.Join(fs.workDir, path)
	return ioutil.ReadFile(fname)

}

func (fs *filesystem) NewSession(session string) OSSession {
	return &filesystemSession{sessName: session, workDir: fs.workDir}
}

func (fs *filesystem) GetSession(path string) *filesystemSession {
	return (fs.NewSession(path)).(*filesystemSession)
}

func (sess *filesystemSession) IsExternal() bool {
	return false
}

func (sess *filesystemSession) SaveData(name string, data []byte) (string, error) {
	fname := filepath.Join(sess.workDir, sess.sessName, name)
	dir := filepath.Dir(fname)
	err := os.MkdirAll(dir, 0755) // noop if dir exists
	if err != nil {
		return "", err
	}
	err = ioutil.WriteFile(fname, data, 0644)
	if err != nil {
		return "", err
	}
	// TODO prob breaks lookups on Windows.... normalize at some point? GetData?
	return name, nil
}

func (sess *filesystemSession) GetData(name string) ([]byte, error) {
	fname := filepath.Join(sess.workDir, sess.sessName, name)
	return ioutil.ReadFile(fname)
}

func (sess *filesystemSession) EndSession() {
	// this makes me nervous ...............
	// if session is empty, could be removing private keys ...........
	// TODO fix fix fix
	os.RemoveAll(filepath.Join(sess.workDir, sess.sessName))
}

func (sess *filesystemSession) GetInfo() *net.OSInfo {
	return nil
}
