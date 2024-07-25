//go:build tools
// +build tools

package tools

import (
	_ "github.com/ethereum/go-ethereum/cmd/abigen"
	_ "github.com/golang/mock/mockgen"
)
