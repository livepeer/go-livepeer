package starter

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/livepeer/go-livepeer/common"
	"github.com/livepeer/go-livepeer/core"
	"github.com/livepeer/go-livepeer/eth"
	lpTypes "github.com/livepeer/go-livepeer/eth/types"
	"github.com/livepeer/go-livepeer/pm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupOrchestrator(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	dbh, dbraw, err := common.TempDB(t)
	require.Nil(err)

	defer dbh.Close()
	defer dbraw.Close()

	orch := pm.RandAddress()

	stubEthClient := &eth.StubClient{
		Orch: &lpTypes.Transcoder{
			Address:           orch,
			ActivationRound:   big.NewInt(5),
			DeactivationRound: big.NewInt(10),
		},
		TranscoderAddress: orch,
	}

	n, err := core.NewLivepeerNode(stubEthClient, "", dbh)
	require.Nil(err)

	err = setupOrchestrator(n, orch)
	assert.Nil(err)

	orchs, err := dbh.SelectOrchs(&common.DBOrchFilter{
		Addresses: []ethcommon.Address{orch},
	})
	assert.Nil(err)
	assert.Len(orchs, 1)
	assert.Equal(orchs[0].ActivationRound, int64(5))
	assert.Equal(orchs[0].DeactivationRound, int64(10))

	// test eth.GetTranscoder error
	stubEthClient.Err = errors.New("GetTranscoder error")
	err = setupOrchestrator(n, orch)
	assert.EqualError(err, "GetTranscoder error")
}

func TestIsLocalURL(t *testing.T) {
	assert := assert.New(t)

	// Test url.ParseRequestURI error
	_, err := isLocalURL("127.0.0.1:8935")
	assert.NotNil(err)

	// Test loopback URLs
	isLocal, err := isLocalURL("https://127.0.0.1:8935")
	assert.Nil(err)
	assert.True(isLocal)
	isLocal, err = isLocalURL("https://127.0.0.2:8935")
	assert.Nil(err)
	assert.True(isLocal)

	// Test localhost URL
	isLocal, err = isLocalURL("https://localhost:8935")
	assert.Nil(err)
	assert.True(isLocal)

	// Test non-local URL
	isLocal, err = isLocalURL("https://0.0.0.0:8935")
	assert.Nil(err)
	assert.False(isLocal)
	isLocal, err = isLocalURL("https://7.7.7.7:8935")
	assert.Nil(err)
	assert.False(isLocal)
}

func TestParseGetGatewayPrices(t *testing.T) {
	assert := assert.New(t)

	// TODO: Keep checking old field for backwards compatibility, remove in future
	jsonTemplate := `{"%s":[{"ethaddress":"0x0000000000000000000000000000000000000000","priceperunit":1000,"pixelsperunit":1}, {"ethaddress":"0x1000000000000000000000000000000000000000","priceperunit":2000,"pixelsperunit":3}]}`
	testCases := []string{"gateways", "broadcasters"}

	for _, tc := range testCases {
		jsonStr := fmt.Sprintf(jsonTemplate, tc)

		prices := getGatewayPrices(jsonStr)
		assert.NotNil(prices)
		assert.Equal(2, len(prices))

		price1 := new(big.Rat).Quo(prices[0].PricePerUnit, prices[0].PixelsPerUnit)
		price2 := new(big.Rat).Quo(prices[1].PricePerUnit, prices[1].PixelsPerUnit)
		assert.Equal(big.NewRat(1000, 1), price1)
		assert.Equal(big.NewRat(2000, 3), price2)
	}
}

func TestMaxPricePerCapability(t *testing.T) {
	assert := assert.New(t)

	jsonTemplate := `{"capabilities_prices": [ {"pipeline": "text-to-image", "model_id": "stabilityai/sd-turbo", "price_per_unit": 1000, "pixels_per_unit": 1}, {"pipeline": "image-to-video", "model_id": "default", "price_per_unit": 2000, "pixels_per_unit": 3}, {"pipeline": "image-to-image", "price_per_unit": 3000, "pixels_per_unit": 1} ] }`

	prices := getCapabilityPrices(jsonTemplate)
	assert.NotNil(prices)
	assert.Equal(3, len(prices))

	// Confirm Pipeline and ModelID is parsed correctly
	assert.Equal(prices[0].Pipeline, "text-to-image")
	assert.Equal(prices[1].Pipeline, "image-to-video")
	assert.Equal(prices[0].ModelID, "stabilityai/sd-turbo")
	assert.Equal(prices[1].ModelID, "default")

	// Confirm prices are parsed correctly
	price1 := new(big.Rat).Quo(prices[0].PricePerUnit, prices[0].PixelsPerUnit)
	price2 := new(big.Rat).Quo(prices[1].PricePerUnit, prices[1].PixelsPerUnit)
	assert.NotEqual(price1, price2)
	assert.Equal(big.NewRat(1000, 1), price1)
	assert.Equal(big.NewRat(2000, 3), price2)

	// Confirm modelID is "default" if not set and price set correctly
	assert.Equal(prices[2].ModelID, "default")
	price3 := new(big.Rat).Quo(prices[2].PricePerUnit, prices[2].PixelsPerUnit)
	assert.Equal(big.NewRat(3000, 1), price3)
}

// Address provided to keystore file
func TestParse_ParseEthKeystorePathValidFile(t *testing.T) {
	assert := assert.New(t)
	tempDir := t.TempDir()

	//Test without 0x in address
	var addr = "0000000000000000000000000000000000000001"
	var fname = "UTC--2023-01-05T00-46-15.503776013Z--" + addr
	file1, err := os.CreateTemp(tempDir, fname)
	if err != nil {
		panic(err)
	}
	defer os.Remove(fname)
	file1.WriteString("{\"address\":\"" + addr + "\",\"crypto\":{\"cipher\":\"1\",\"ciphertext\":\"1\",\"cipherparams\":{\"iv\":\"1\"},\"kdf\":\"scrypt\",\"kdfparams\":{\"dklen\":32,\"n\":1,\"p\":1,\"r\":8,\"salt\":\"1\"},\"mac\":\"1\"},\"id\":\"1\",\"version\":3}")

	var keystoreInfo keystorePath
	keystoreInfo, _ = parseEthKeystorePath(file1.Name())

	assert.Empty(keystoreInfo.path)
	assert.NotEmpty(keystoreInfo.address)
	assert.True(ethcommon.BytesToAddress(ethcommon.FromHex(addr)) == keystoreInfo.address)
	assert.True(err == nil)

	//Test with 0x in address
	var hexAddr = "0x0000000000000000000000000000000000000001"
	var fname2 = "UTC--2023-01-05T00-46-15.503776013Z--" + hexAddr
	file2, err := os.CreateTemp(tempDir, fname2)
	if err != nil {
		panic(err)
	}
	defer os.Remove(fname2)
	file2.WriteString("{\"address\":\"" + addr + "\",\"crypto\":{\"cipher\":\"1\",\"ciphertext\":\"1\",\"cipherparams\":{\"iv\":\"1\"},\"kdf\":\"scrypt\",\"kdfparams\":{\"dklen\":32,\"n\":1,\"p\":1,\"r\":8,\"salt\":\"1\"},\"mac\":\"1\"},\"id\":\"1\",\"version\":3}")

	keystoreInfo, _ = parseEthKeystorePath(file1.Name())
	assert.Empty(keystoreInfo.path)
	assert.NotEmpty(keystoreInfo.address)
	assert.True(ethcommon.BytesToAddress(ethcommon.FromHex(addr)) == keystoreInfo.address)
	assert.True(err == nil)
}

func TestParse_ParseEthKeystorePathValidDirectory(t *testing.T) {
	assert := assert.New(t)
	tempDir := t.TempDir()

	var keystoreInfo keystorePath
	keystoreInfo, err := parseEthKeystorePath(tempDir)
	assert.NotEmpty(keystoreInfo.path)
	assert.Empty(keystoreInfo.address)
	assert.True(err == nil)
}

// Keystore file exists, but address cannot be parsed
func TestParse_ParseEthKeystorePathInvalidJSON(t *testing.T) {
	assert := assert.New(t)
	tempDir := t.TempDir()

	//Create test file
	var addr = "0x0000000000000000000000000000000000000001"
	var fname = "UTC--2023-01-05T00-46-15.503776013Z--" + addr
	badJsonfile, err := os.CreateTemp(tempDir, fname)
	if err != nil {
		panic(err)
	}

	defer os.Remove(fname)
	badJsonfile.WriteString("{{\"address_broken_json\":\"" + addr + "\"}")

	var keystoreInfo keystorePath
	keystoreInfo, err = parseEthKeystorePath(badJsonfile.Name())
	assert.Empty(keystoreInfo.path)
	assert.Empty(keystoreInfo.address)
	assert.True(err.Error() == "error parsing address from keyfile")
}

// Keystore path or file doesn't exist
func TestParse_ParseEthKeystorePathFileNotFound(t *testing.T) {
	assert := assert.New(t)
	tempDir := t.TempDir()
	var keystoreInfo keystorePath
	//Test missing key file
	keystoreInfo, err := parseEthKeystorePath(filepath.Join(tempDir, "missing_keyfile"))
	assert.Empty(keystoreInfo.path)
	assert.Empty(keystoreInfo.address)
	assert.True(err.Error() == "provided -ethKeystorePath was not found")

	//Test missing key file directory
	keystoreInfo, err = parseEthKeystorePath(filepath.Join("missing_directory", "missing_keyfile"))
	assert.Empty(keystoreInfo.path)
	assert.Empty(keystoreInfo.address)
	assert.True(err.Error() == "provided -ethKeystorePath was not found")
}

func TestUpdatePerfScore(t *testing.T) {
	b, err := os.ReadFile("test_fixtures/perf_stats.json") // just pass the file name
	assert.NoError(t, err, "Unable to open test fixture `test_fixtures/perf_stats.json`")

	perfStatsResp := string(b)
	scores := &common.PerfScore{Scores: map[ethcommon.Address]float64{
		// some previous data
		ethcommon.HexToAddress("0x001ffe939761eea3f37dd2223bd08401a3848bf3"): 0.11,
	}}

	updatePerfScore("LAX", []byte(perfStatsResp), scores)

	expScores := map[ethcommon.Address]float64{
		ethcommon.HexToAddress("0x001ffe939761eea3f37dd2223bd08401a3848bf3"): 0.326224769938329,
		ethcommon.HexToAddress("0x00803b76dc924ceabf4380a6f9edc2ddd3c90f38"): 0.3133182335267256,
	}
	require.Equal(t, expScores, scores.Scores)
}

func TestParsePricePerUnit(t *testing.T) {
	tests := []struct {
		name             string
		pricePerUnitStr  string
		expectedPrice    *big.Rat
		expectedCurrency string
		expectError      bool
	}{
		{
			name:             "Valid input with integer price",
			pricePerUnitStr:  "100USD",
			expectedPrice:    big.NewRat(100, 1),
			expectedCurrency: "USD",
			expectError:      false,
		},
		{
			name:             "Valid input with fractional price",
			pricePerUnitStr:  "0.13USD",
			expectedPrice:    big.NewRat(13, 100),
			expectedCurrency: "USD",
			expectError:      false,
		},
		{
			name:             "Valid input with decimal price",
			pricePerUnitStr:  "99.99EUR",
			expectedPrice:    big.NewRat(9999, 100),
			expectedCurrency: "EUR",
			expectError:      false,
		},
		{
			name:             "Lower case currency",
			pricePerUnitStr:  "99.99eur",
			expectedPrice:    big.NewRat(9999, 100),
			expectedCurrency: "eur",
			expectError:      false,
		},
		{
			name:             "Currency with numbers",
			pricePerUnitStr:  "420DOG3",
			expectedPrice:    big.NewRat(420, 1),
			expectedCurrency: "DOG3",
			expectError:      false,
		},
		{
			name:             "No specified currency, empty currency",
			pricePerUnitStr:  "100",
			expectedPrice:    big.NewRat(100, 1),
			expectedCurrency: "",
			expectError:      false,
		},
		{
			name:             "Explicit wei currency",
			pricePerUnitStr:  "100wei",
			expectedPrice:    big.NewRat(100, 1),
			expectedCurrency: "wei",
			expectError:      false,
		},
		{
			name:             "Invalid number",
			pricePerUnitStr:  "abcUSD",
			expectedPrice:    nil,
			expectedCurrency: "",
			expectError:      true,
		},
		{
			name:             "Negative price",
			pricePerUnitStr:  "-100USD",
			expectedPrice:    nil,
			expectedCurrency: "",
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price, currency, err := parsePricePerUnit(tt.pricePerUnitStr)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.True(t, tt.expectedPrice.Cmp(price) == 0)
				assert.Equal(t, tt.expectedCurrency, currency)
			}
		})
	}
}

func TestPrintConfigRedaction(t *testing.T) {
	assert := assert.New(t)

	// Create a config with sensitive values
	cfg := DefaultLivepeerConfig()
	testPassword := "secretpassword123"
	testApiKey := "api-key-abc123"
	testOrchSecret := "orch-secret-456"
	testServiceAddr := "127.0.0.1:8936"

	cfg.EthPassword = &testPassword
	cfg.LiveAIAuthApiKey = &testApiKey
	cfg.OrchSecret = &testOrchSecret
	cfg.ServiceAddr = &testServiceAddr

	// Capture the output
	var buf []byte
	writer := &testWriter{buf: &buf}
	cfg.PrintConfig(writer)

	output := string(buf)

	// Verify sensitive values are redacted
	assert.NotContains(output, testPassword, "EthPassword should be redacted")
	assert.NotContains(output, testApiKey, "LiveAIAuthApiKey should be redacted")
	assert.NotContains(output, testOrchSecret, "OrchSecret should be redacted")
	assert.Contains(output, "***", "Should contain redacted placeholder")

	// Verify non-sensitive values are still shown
	assert.Contains(output, testServiceAddr, "ServiceAddr should not be redacted")
}

// Helper struct to capture output for testing
type testWriter struct {
	buf *[]byte
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}
