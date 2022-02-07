package eth

import (
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"math"
	"math/big"
	"reflect"
	"regexp"
	"strings"
)

const DefaultMaxDecimals = 18
const percDivisor = 1000000
const percDivisorMinter = 1000000000

var (
	BlocksUntilFirstClaimDeadline = big.NewInt(200)
	maxUint256                    = new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil), big.NewInt(1))
)

func FormatUnits(baseAmount *big.Int, name string) string {
	if baseAmount == nil {
		return "0 " + name
	}

	if baseAmount.Cmp(big.NewInt(10000000000000000)) == -1 {
		switch name {
		case "ETH":
			return fmt.Sprintf("%v WEI", baseAmount)
		default:
			return fmt.Sprintf("%v LPTU", baseAmount)
		}
	} else {
		switch name {
		case "ETH":
			return fmt.Sprintf("%v ETH", FromBaseAmount(baseAmount, DefaultMaxDecimals))
		default:
			return fmt.Sprintf("%v LPT", FromBaseAmount(baseAmount, DefaultMaxDecimals))
		}
	}
}

func FormatPercMinter(value *big.Int) string {
	pMultiplier := percDivisorMinter / 100.0
	return fmt.Sprintf("%v", float64(value.Int64())/pMultiplier)
}

func FormatPerc(value *big.Int) string {
	perc := ToPerc(value)

	return fmt.Sprintf("%v", perc)
}

func ToPerc(value *big.Int) float64 {
	pMultiplier := percDivisor / 100.0

	return float64(value.Int64()) / pMultiplier
}

func FromPerc(perc float64) *big.Int {
	return fromPerc(perc, big.NewFloat(percDivisor/100.0))
}

func FromPercOfUint256(perc float64) *big.Int {
	multiplier := new(big.Float).SetInt(new(big.Int).Div(maxUint256, big.NewInt(100)))
	return fromPerc(perc, multiplier)
}

func IsNullAddress(addr ethcommon.Address) bool {
	return addr == ethcommon.Address{}
}

func fromPerc(perc float64, multiplier *big.Float) *big.Int {
	floatRes := new(big.Float).Mul(big.NewFloat(perc), multiplier)
	intRes, _ := floatRes.Int(nil)
	return intRes
}

func ToBaseAmount(v string, maxDecimals int) (*big.Int, error) {
	// check that string is a float represented as a string with "." as seperator
	ok, err := regexp.MatchString("^[-+]?[0-9]*.?[0-9]+$", v)
	if !ok || err != nil {
		return nil, fmt.Errorf("submitted value %v is not a valid float", v)
	}
	splitNum := strings.Split(v, ".")
	// If '.' is absent from string, add an empty string to become the decimal part
	if len(splitNum) == 1 {
		splitNum = append(splitNum, "")
	}
	intPart := splitNum[0]
	decPart := splitNum[1]

	// If decimal part is longer than 18 decimals return an error
	if len(decPart) > maxDecimals {
		return nil, fmt.Errorf("submitted value has more than 18 decimals")
	}

	// If decimal part is shorter than 18 digits, pad with "0"
	for len(decPart) < maxDecimals {
		decPart += "0"
	}

	// Combine intPart and decPart into a big.Int
	baseAmount, ok := new(big.Int).SetString(intPart+decPart, 10)
	if !ok {
		return nil, fmt.Errorf("unable to convert floatString %v into big.Int", intPart+decPart)
	}

	return baseAmount, nil
}

func FromBaseAmount(v *big.Int, maxDecimals int) string {
	baseAmount := v.String()

	// if length < 18 leftPad with zeros
	for len(baseAmount) < maxDecimals {
		baseAmount = "0" + baseAmount
	}

	// split decPart and intPart
	intPart := baseAmount[:len(baseAmount)-maxDecimals]
	decPart := baseAmount[len(baseAmount)-maxDecimals:]

	// if no int part, add "0"
	if len(intPart) == 0 {
		intPart = "0"
	}

	// remove right padded 0's from decPart
	decPart = strings.TrimRight(decPart, "0")

	// if there are no decimals just return the integer part
	if len(decPart) == 0 {
		return intPart
	}

	return intPart + "." + decPart
}

func parseABI(abiString string) (abi.ABI, error) {
	return abi.JSON(strings.NewReader(abiString))
}

func decodeTxParams(abi *abi.ABI, v map[string]interface{}, data []byte) error {
	m, err := abi.MethodById(data[:4])
	if err != nil {
		return err
	}
	if err := m.Inputs.UnpackIntoMap(v, data[4:]); err != nil {
		return err
	}
	for k, val := range v {
		v[k] = ethTypeToStringyType(val)
	}
	return nil
}

func ethTypeToStringyType(v interface{}) interface{} {
	val := reflect.Indirect(reflect.ValueOf(v))

	switch vTy := val.Interface().(type) {
	case []byte:
		return "0x" + ethcommon.Bytes2Hex(vTy)
	case [32]byte:
		return fmt.Sprintf("0x%x", vTy)
	case ethcommon.Address:
		return vTy.Hex()
	case ethcommon.Hash:
		return "0x" + vTy.Hex()
	case big.Int:
		return vTy.String()
	default:
		return handleComplexEthType(val)
	}
}

func handleComplexEthType(val reflect.Value) interface{} {
	switch val.Kind() {
	// tuple
	case reflect.Struct:
		vString := "{"
		for i := 0; i < val.NumField(); i++ {
			vString += fmt.Sprintf(" %v", val.Type().Field(i).Name)
			vString += ": "
			vString += fmt.Sprintf("%v ", ethTypeToStringyType(val.Field(i).Interface()))
		}
		vString += "}"
		return vString
	case reflect.Array:
		return handleEthSlice(val)
	case reflect.Slice:
		return handleEthSlice(val)
	default:
		return val.Interface()
	}
}

func handleEthSlice(val reflect.Value) string {
	if val.Kind() != reflect.Array && val.Kind() != reflect.Slice {
		return ""
	}
	vString := "["
	for i := 0; i < val.Len(); i++ {
		vString += fmt.Sprintf(" %v ", ethTypeToStringyType(val.Index(i).Interface()))
	}
	vString += "]"
	return vString
}

// FromWei converts a wei amount to another denomination
// e.g. FromWei(1e18, params.Ether) = 1
func FromWei(amount *big.Int, to int) string {
	maxDecimals := int(math.Log10(float64(to)))
	return FromBaseAmount(amount, maxDecimals)
}

// ToWei converts an amount in another denomination to the corresponding amount in wei
// e.g. ToWei("1", params.Ether) = 1e18
func ToWei(amount string, from int) (*big.Int, error) {
	maxDecimals := int(math.Log10(float64(from)))
	return ToBaseAmount(amount, maxDecimals)
}
