package worker

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadImageB64DataUrl(t *testing.T) {
	tests := []struct {
		name        string
		dataURL     string
		expectError bool
	}{
		{
			name: "Valid PNG Image",
			dataURL: func() string {
				img := image.NewRGBA(image.Rect(0, 0, 1, 1))
				img.Set(0, 0, color.RGBA{255, 0, 0, 255}) // Set a single red pixel
				var imgBuf bytes.Buffer
				err := png.Encode(&imgBuf, img)
				require.NoError(t, err)

				return "data:image/png;base64," + base64.StdEncoding.EncodeToString(imgBuf.Bytes())
			}(),
			expectError: false,
		},
		{
			name: "Unsupported Image Format",
			dataURL: "data:image/bmp;base64," + base64.StdEncoding.EncodeToString([]byte{
				0x42, 0x4D, // BMP header
				// ... (rest of the BMP data)
			}),
			expectError: true,
		},
		{
			name:        "Invalid Data URL",
			dataURL:     "invalid-data-url",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := ReadImageB64DataUrl(tt.dataURL, &buf)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, buf.Bytes())
			}
		})
	}
}

func TestSaveImageB64DataUrl(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255}) // Set a single red pixel
	var imgBuf bytes.Buffer
	err := png.Encode(&imgBuf, img)
	require.NoError(t, err)
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(imgBuf.Bytes())

	outputPath := "test_output.png"
	defer os.Remove(outputPath)

	err = SaveImageB64DataUrl(dataURL, outputPath)
	require.NoError(t, err)

	// Verify that the file was created and is not empty
	fileInfo, err := os.Stat(outputPath)
	require.NoError(t, err)
	require.False(t, fileInfo.IsDir())
	require.NotZero(t, fileInfo.Size())
}

func TestReadAudioB64DataUrl(t *testing.T) {
	// Create a sample audio data and encode it as a data URL
	audioData := []byte{0x00, 0x01, 0x02, 0x03, 0x04}
	dataURL := "data:audio/wav;base64," + base64.StdEncoding.EncodeToString(audioData)

	var buf bytes.Buffer
	err := ReadAudioB64DataUrl(dataURL, &buf)
	require.NoError(t, err)
	require.Equal(t, audioData, buf.Bytes())
}
