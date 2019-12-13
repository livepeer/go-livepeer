package common

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPassNoFileExists(t *testing.T) {
	assert := assert.New(t)

	input := "/tmp/../../../nothing"
	expectedOutput := "/tmp/../../../nothing"

	// GetPass should return the string it was supplied
	output, err := GetPass(input)

	assert.Nil(err)
	// GetPass should return the originaly supplied string
	assert.Equal(expectedOutput, output)
}

func TestGetPassDirectoryExists(t *testing.T) {
	assert := assert.New(t)

	input := "/tmp"
	expectedOutput := "/tmp"

	// GetPass should return the string it was supplied
	output, err := GetPass(input)

	assert.NotNil(err)
	// GetPass should return the originaly supplied string
	assert.Equal(expectedOutput, output)
}

func TestGetPassEmptyFileExists(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	tmpFile := "TestGetPassEmptyFileExists.txt"
	expectedOutput := "TestGetPassEmptyFileExists.txt"

	emptyFile, err := os.Create(tmpFile)
	emptyFile.Close()
	require.Nil(err)

	defer func() {
		err := os.Remove(tmpFile)
		require.Nil(err)
	}()

	// GetPass should return an error
	output, err := GetPass(tmpFile)

	assert.NotNil(err)
	// GetPass should return the originaly supplied string
	assert.Equal(expectedOutput, output)
}

func TestGetPassFileExistsOneLine(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	input := `something`
	expectedOutput := "something"

	tmpFile := "TestGetPassFileExistsOneLine.txt"

	file, err := os.Create(tmpFile)
	require.Nil(err)
	_, err = file.WriteString(fmt.Sprintf("%s\n", input))
	file.Close()
	require.Nil(err)

	defer func() {
		err := os.Remove(tmpFile)
		require.Nil(err)
	}()

	output, err := GetPass(tmpFile)

	assert.Nil(err)
	// GetPass should the first line of the text file
	assert.Equal(expectedOutput, output)
}

func TestGetPassFileExistsMultiline(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	input :=
		`something
somethingelse`
	expectedOutput := "something"

	tmpFile := "TestGetPassFileExistsMultiline.txt"

	file, err := os.Create(tmpFile)
	require.Nil(err)
	_, err = file.WriteString(fmt.Sprintf("%s\n", input))
	file.Close()
	require.Nil(err)

	defer func() {
		err := os.Remove(tmpFile)
		require.Nil(err)
	}()

	output, err := GetPass(tmpFile)

	assert.Nil(err)
	// GetPass should the first line of the text file
	assert.Equal(expectedOutput, output)
}
