package eth

import (
	"fmt"
	"strconv"
	"strings"

	ethcommon "github.com/ethereum/go-ethereum/common"
	sdk "github.com/tkhq/go-sdk"
	"github.com/tkhq/go-sdk/pkg/api/client/wallets"
	"github.com/tkhq/go-sdk/pkg/api/models"
	"github.com/tkhq/go-sdk/pkg/util"
)

// TurnkeyWalletAccount is one Ethereum account managed by Turnkey.
type TurnkeyWalletAccount struct {
	OrganizationID string
	WalletID       string
	AccountID      string
	Address        ethcommon.Address
}

// DefaultEthereumDerivationPath is the standard first-account path for Ethereum (BIP-44).
const DefaultEthereumDerivationPath = `m/44'/60'/0'/0/0`

func ethereumWalletAccountParams(path string) *models.WalletAccountParams {
	return &models.WalletAccountParams{
		AddressFormat: models.AddressFormatEthereum.Pointer(),
		Curve:         models.CurveSecp256k1.Pointer(),
		PathFormat:    models.PathFormatBip32.Pointer(),
		Path:          &path,
	}
}

// ListTurnkeyEthereumAccounts returns all Ethereum-format wallet accounts in the organization.
func ListTurnkeyEthereumAccounts(c *sdk.Client, orgID string) ([]TurnkeyWalletAccount, error) {
	if c == nil {
		return nil, fmt.Errorf("turnkey client is nil")
	}
	params := wallets.NewGetWalletAccountsParams().WithBody(&models.GetWalletAccountsRequest{
		OrganizationID: &orgID,
	})
	resp, err := c.V0().Wallets.GetWalletAccounts(params, c.Authenticator)
	if err != nil {
		return nil, err
	}
	payload := resp.GetPayload()
	if payload == nil {
		return nil, nil
	}
	var out []TurnkeyWalletAccount
	for _, a := range payload.Accounts {
		if a == nil || a.Address == nil || a.AddressFormat == nil {
			continue
		}
		if *a.AddressFormat != models.AddressFormatEthereum {
			continue
		}
		wid := ""
		if a.WalletID != nil {
			wid = *a.WalletID
		}
		aid := ""
		if a.WalletAccountID != nil {
			aid = *a.WalletAccountID
		}
		out = append(out, TurnkeyWalletAccount{
			OrganizationID: orgID,
			WalletID:       wid,
			AccountID:      aid,
			Address:        ethcommon.HexToAddress(*a.Address),
		})
	}
	return out, nil
}

// TurnkeyCreateWallet creates a new HD wallet with one derived Ethereum account.
func TurnkeyCreateWallet(c *sdk.Client, orgID, walletName string) (walletID string, firstAddress ethcommon.Address, err error) {
	if c == nil {
		return "", ethcommon.Address{}, fmt.Errorf("turnkey client is nil")
	}
	act := models.CreateWalletRequestTypeACTIVITYTYPECREATEWALLET
	path := DefaultEthereumDerivationPath
	req := &models.CreateWalletRequest{
		OrganizationID: &orgID,
		TimestampMs:    util.RequestTimestamp(),
		Type:           &act,
		Parameters: &models.CreateWalletIntent{
			WalletName: &walletName,
			Accounts: []*models.WalletAccountParams{
				ethereumWalletAccountParams(path),
			},
		},
	}
	resp, err := c.V0().Wallets.CreateWallet(wallets.NewCreateWalletParams().WithBody(req), c.Authenticator)
	if err != nil {
		return "", ethcommon.Address{}, err
	}
	if resp.Payload == nil || resp.Payload.Activity == nil || resp.Payload.Activity.Result == nil {
		return "", ethcommon.Address{}, fmt.Errorf("turnkey CreateWallet: empty activity result")
	}
	res := resp.Payload.Activity.Result.CreateWalletResult
	if res == nil || res.WalletID == nil || len(res.Addresses) == 0 {
		return "", ethcommon.Address{}, fmt.Errorf("turnkey CreateWallet: missing walletId or addresses")
	}
	return *res.WalletID, ethcommon.HexToAddress(res.Addresses[0]), nil
}

// TurnkeyCreateWalletAccount derives a new Ethereum account on an existing wallet using the next BIP-44 index.
func TurnkeyCreateWalletAccount(c *sdk.Client, orgID, walletID string) (addr ethcommon.Address, err error) {
	if c == nil {
		return ethcommon.Address{}, fmt.Errorf("turnkey client is nil")
	}
	idx, err := nextEthereumDerivationIndex(c, orgID, walletID)
	if err != nil {
		return ethcommon.Address{}, err
	}
	path := fmt.Sprintf(`m/44'/60'/0'/0/%d`, idx)
	act := models.CreateWalletAccountsRequestTypeACTIVITYTYPECREATEWALLETACCOUNTS
	req := &models.CreateWalletAccountsRequest{
		OrganizationID: &orgID,
		TimestampMs:    util.RequestTimestamp(),
		Type:           &act,
		Parameters: &models.CreateWalletAccountsIntent{
			WalletID: &walletID,
			Accounts: []*models.WalletAccountParams{
				ethereumWalletAccountParams(path),
			},
		},
	}
	resp, err := c.V0().Wallets.CreateWalletAccounts(wallets.NewCreateWalletAccountsParams().WithBody(req), c.Authenticator)
	if err != nil {
		return ethcommon.Address{}, err
	}
	if resp.Payload == nil || resp.Payload.Activity == nil || resp.Payload.Activity.Result == nil {
		return ethcommon.Address{}, fmt.Errorf("turnkey CreateWalletAccounts: empty activity result")
	}
	res := resp.Payload.Activity.Result.CreateWalletAccountsResult
	if res == nil || len(res.Addresses) == 0 {
		return ethcommon.Address{}, fmt.Errorf("turnkey CreateWalletAccounts: missing addresses")
	}
	return ethcommon.HexToAddress(res.Addresses[0]), nil
}

func nextEthereumDerivationIndex(c *sdk.Client, orgID, walletID string) (int, error) {
	wid := walletID
	params := wallets.NewGetWalletAccountsParams().WithBody(&models.GetWalletAccountsRequest{
		OrganizationID: &orgID,
		WalletID:       &wid,
	})
	resp, err := c.V0().Wallets.GetWalletAccounts(params, c.Authenticator)
	if err != nil {
		return 0, err
	}
	maxIdx := -1
	if resp.Payload != nil {
		for _, a := range resp.Payload.Accounts {
			if a == nil || a.Path == nil || a.AddressFormat == nil {
				continue
			}
			if *a.AddressFormat != models.AddressFormatEthereum {
				continue
			}
			idx, ok := parseBIP44EthereumIndex(*a.Path)
			if ok && idx > maxIdx {
				maxIdx = idx
			}
		}
	}
	return maxIdx + 1, nil
}

func parseBIP44EthereumIndex(path string) (int, bool) {
	path = strings.TrimSpace(path)
	const prefix = `m/44'/60'/0'/0/`
	if !strings.HasPrefix(path, prefix) {
		return 0, false
	}
	tail := strings.TrimSpace(path[len(prefix):])
	if tail == "" {
		return 0, false
	}
	n, err := strconv.Atoi(tail)
	if err != nil {
		return 0, false
	}
	return n, true
}

// TurnkeyAddressMap builds a lookup from Ethereum address to wallet metadata.
func TurnkeyAddressMap(accts []TurnkeyWalletAccount) map[ethcommon.Address]TurnkeyWalletAccount {
	m := make(map[ethcommon.Address]TurnkeyWalletAccount)
	for _, a := range accts {
		m[a.Address] = a
	}
	return m
}
