package transcoder

type Transcoder interface {
	Transcode(d []byte) ([][]byte, error)
}
