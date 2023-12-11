package common

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFromFileNoFileExists(t *testing.T) {
	assert := assert.New(t)

	input := "/tmp/../../../nothing"
	expectedOutput := "/tmp/../../../nothing"

	// ReadFromFile should return the string it was supplied
	output, err := ReadFromFile(input)

	assert.NotNil(err)
	// ReadFromFile should return the originally supplied string
	assert.Equal(expectedOutput, output)
}

func TestReadFromFileDirectoryExists(t *testing.T) {
	assert := assert.New(t)

	input := "/tmp"
	expectedOutput := "/tmp"

	// ReadFromFile should return the string it was supplied
	output, err := ReadFromFile(input)

	assert.NotNil(err)
	// ReadFromFile should return the originally supplied string
	assert.Equal(expectedOutput, output)
}

func TestReadFromFileEmptyFileExists(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	tmpFile := "TestReadFromFileEmptyFileExists.txt"
	expectedOutput := "TestReadFromFileEmptyFileExists.txt"

	emptyFile, err := os.Create(tmpFile)
	emptyFile.Close()
	require.Nil(err)

	defer func() {
		err := os.Remove(tmpFile)
		require.Nil(err)
	}()

	// ReadFromFile should return an error
	output, err := ReadFromFile(tmpFile)

	assert.NotNil(err)
	// ReadFromFile should return the originally supplied string
	assert.Equal(expectedOutput, output)
}

func TestReadFromFile_FileExistsOneLine(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	input := `something`
	expectedOutput := "something"

	tmpFile := "TestReadFromFile_FileExistsOneLine.txt"

	file, err := os.Create(tmpFile)
	require.Nil(err)
	_, err = file.WriteString(fmt.Sprintf("%s\n", input))
	file.Close()
	require.Nil(err)

	defer func() {
		err := os.Remove(tmpFile)
		require.Nil(err)
	}()

	output, err := ReadFromFile(tmpFile)

	assert.Nil(err)
	// ReadFromFile should the first line of the text file
	assert.Equal(expectedOutput, output)
}

// Check that we can reliably read a single line value from the file, even when it ends in a newline / whitespace
func TestReadFromFile_FileExistsEndsInNewline(t *testing.T) {
	input :=
		`something
`
	f, err := os.CreateTemp(os.TempDir(), "TestReadFromFile_FileExistsEndsInNewline*")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	_, err = f.WriteString(input)
	require.NoError(t, err)

	output, err := ReadFromFile(f.Name())
	require.NoError(t, err)
	require.Equal(t, "something", output)
}

func TestReadFromFile_FileExistsMultiline(t *testing.T) {
	require := require.New(t)

	input :=
		`something
somethingelse`

	tmpFile := "TestReadFromFile_FileExistsMultiline.txt"

	file, err := os.Create(tmpFile)
	require.Nil(err)
	_, err = file.WriteString(fmt.Sprintf("%s\n", input))
	file.Close()
	require.Nil(err)

	defer func() {
		err := os.Remove(tmpFile)
		require.Nil(err)
	}()

	output, err := ReadFromFile(tmpFile)

	require.NoError(err)
	// ReadFromFile should the first line of the text file
	require.Equal(input, output)
}
