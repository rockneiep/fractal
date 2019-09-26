package plugin

import (
	"errors"
	"math/big"

	"github.com/fractalplatform/fractal/accountmanager"
	"github.com/fractalplatform/fractal/common"
	"github.com/fractalplatform/fractal/types"
)

func CreateAccount(context *Context) ([]byte, uint64, error) {
	return nil, context.Gas, nil
}

func GetAccountBalanceByID(account *accountmanager.AccountManager, accountName common.Name, assetID uint64, typeID uint64) (*big.Int, error) {
	return account.GetAccountBalanceByID(accountName, assetID, typeID)
}

func GetNonce(account *accountmanager.AccountManager, accountName common.Name) (uint64, error) {
	return account.GetNonce(accountName)
}

func CompareNonce(account *accountmanager.AccountManager, accountName common.Name, actionNonce uint64) error {
	nonce, err := account.GetNonce(accountName)
	if err != nil {
		return err
	}
	if nonce < actionNonce {
		return errors.New("nonce too high")
	} else if nonce > actionNonce {
		return errors.New("nonce too low")
	}
	return nil
}

func UpdateNonce(account *accountmanager.AccountManager, accountName common.Name) error {
	nonce, err := account.GetNonce(accountName)
	if err != nil {
		return err
	}
	err = account.SetNonce(accountName, nonce+1)
	if err != nil {
		return err
	}
	return nil
}

func SetNonce(account *accountmanager.AccountManager, accountName common.Name, nonce uint64) error {
	return account.SetNonce(accountName, nonce)
}

func TransferAsset(account *accountmanager.AccountManager, from common.Name, to common.Name, assetID uint64, value *big.Int) error {
	return account.TransferAsset(from, to, assetID, value)
}

func RecoverTx(account *accountmanager.AccountManager, signer types.Signer, tx *types.Transaction) error {
	return account.RecoverTx(signer, tx)
}

func GetAuthorVersion(account *accountmanager.AccountManager, name common.Name) (common.Hash, error) {
	return account.GetAuthorVersion(name)
}
