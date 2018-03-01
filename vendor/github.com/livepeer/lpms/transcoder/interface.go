package transcoder

type Transcoder interface {
	Transcode(fname string) ([][]byte, error)
}
