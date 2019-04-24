// Copyright 2018 The Fractal Team Authors
// This file is part of the fractal project.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package rpcapi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/fractalplatform/fractal/accountmanager"
	"github.com/fractalplatform/fractal/common"
	"github.com/fractalplatform/fractal/params"
	"github.com/fractalplatform/fractal/processor"
	"github.com/fractalplatform/fractal/processor/vm"
	"github.com/fractalplatform/fractal/rawdb"
	"github.com/fractalplatform/fractal/rpc"
	"github.com/fractalplatform/fractal/types"
)

// PublicBlockChainAPI provides an API to access the blockchain.
// It offers only methods that operate on public data that is freely available to anyone.
const (
	defaultGasPrice = 1e9
)

type PublicBlockChainAPI struct {
	b Backend
}

// NewPublicBlockChainAPI creates a new blockchain API.
func NewPublicBlockChainAPI(b Backend) *PublicBlockChainAPI {
	return &PublicBlockChainAPI{b}
}

// GetCurrentBlock returns cureent block.
func (s *PublicBlockChainAPI) GetCurrentBlock(fullTx bool) map[string]interface{} {
	block := s.b.CurrentBlock()
	response := s.rpcOutputBlock(s.b.ChainConfig().ChainID, block, true, fullTx)
	return response
}

// GetBlockByHash returns the requested block. When fullTx is true all transactions in the block are returned in full
// detail, otherwise only the transaction hash is returned.
func (s *PublicBlockChainAPI) GetBlockByHash(ctx context.Context, blockHash common.Hash, fullTx bool) (map[string]interface{}, error) {
	block, err := s.b.GetBlock(ctx, blockHash)
	if block != nil {
		return s.rpcOutputBlock(s.b.ChainConfig().ChainID, block, true, fullTx), nil
	}
	return nil, err
}

// GetBlockByNumber returns the requested block. When blockNr is -1 the chain head is returned. When fullTx is true all
// transactions in the block are returned in full detail, otherwise only the transaction hash is returned.
func (s *PublicBlockChainAPI) GetBlockByNumber(ctx context.Context, blockNr rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	block, err := s.b.BlockByNumber(ctx, blockNr)
	if block != nil {
		response := s.rpcOutputBlock(s.b.ChainConfig().ChainID, block, true, fullTx)
		if blockNr == rpc.PendingBlockNumber {
			// Pending blocks need to nil out a few fields
			for _, field := range []string{"hash", "nonce", "miner"} {
				response[field] = nil
			}
		}
		return response, err
	}
	return nil, err
}

// rpcOutputBlock uses the generalized output filler, then adds the total difficulty field, which requires
// a `PublicBlockchainAPI`.
func (s *PublicBlockChainAPI) rpcOutputBlock(chainID *big.Int, b *types.Block, inclTx bool, fullTx bool) map[string]interface{} {
	fields := RPCMarshalBlock(chainID, b, inclTx, fullTx)
	fields["totalDifficulty"] = s.b.GetTd(b.Hash())
	return fields
}

// GetTransactionByHash returns the transaction for the given hash
func (s *PublicBlockChainAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) *types.RPCTransaction {
	// Try to return an already finalized transaction
	if tx, blockHash, blockNumber, index := rawdb.ReadTransaction(s.b.ChainDb(), hash); tx != nil {
		return tx.NewRPCTransaction(blockHash, blockNumber, index)
	}
	// No finalized transaction, try to retrieve it from the pool
	if tx := s.b.GetPoolTransaction(hash); tx != nil {
		return tx.NewRPCTransaction(common.Hash{}, 0, 0)
	}
	// Transaction unknown, return as such
	return nil
}

// GetTransactionReceipt returns the transaction receipt for the given transaction hash.
func (s *PublicBlockChainAPI) GetTransactionReceipt(ctx context.Context, hash common.Hash) (*types.RPCReceipt, error) {
	tx, blockHash, blockNumber, index := rawdb.ReadTransaction(s.b.ChainDb(), hash)
	if tx == nil {
		return nil, nil
	}

	receipts, err := s.b.GetReceipts(ctx, blockHash)
	if err != nil {
		return nil, err
	}
	if len(receipts) <= int(index) {
		return nil, nil
	}
	receipt := receipts[index]

	return receipt.NewRPCReceipt(blockHash, blockNumber, index, tx), nil
}

func (s *PublicBlockChainAPI) GetBlockAndResultByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.BlockAndResult, error) {
	r := s.b.GetBlockDetailLog(ctx, blockNr)
	if r == nil {
		return nil, nil
	}
	block, err := s.GetBlockByNumber(ctx, blockNr, true)
	r.Block = block
	return r, err
}

func (s *PublicBlockChainAPI) GetTxsByAccount(ctx context.Context, acctName common.Name, blockNr rpc.BlockNumber, lookbackNum uint64) ([]common.Hash, error) {
	filterFn := func(name common.Name) bool {
		return name == acctName
	}

	return s.b.GetTxsByFilter(ctx, filterFn, blockNr, lookbackNum), nil
}

func (s *PublicBlockChainAPI) GetTxsByBloom(ctx context.Context, bloomByte hexutil.Bytes, blockNr rpc.BlockNumber, lookbackNum uint64) ([]common.Hash, error) {
	bloom := types.BytesToBloom(bloomByte)

	filterFn := func(name common.Name) bool {
		return bloom.TestBytes([]byte(name))
	}
	return s.b.GetTxsByFilter(ctx, filterFn, blockNr, lookbackNum), nil
}

func (s *PublicBlockChainAPI) GetInternalTxByAccount(ctx context.Context, acctName common.Name, blockNr rpc.BlockNumber, lookbackNum uint64) ([]*types.DetailTx, error) {
	filterFn := func(name common.Name) bool {
		return name == acctName
	}

	return s.b.GetDetailTxByFilter(ctx, filterFn, blockNr, lookbackNum), nil
}

func (s *PublicBlockChainAPI) GetInternalTxByBloom(ctx context.Context, bloomByte hexutil.Bytes, blockNr rpc.BlockNumber, lookbackNum uint64) ([]*types.DetailTx, error) {
	bloom := types.BytesToBloom(bloomByte)

	filterFn := func(name common.Name) bool {
		return bloom.TestBytes([]byte(name))
	}
	return s.b.GetDetailTxByFilter(ctx, filterFn, blockNr, lookbackNum), nil
}

func (s *PublicBlockChainAPI) GetInternalTxByHash(ctx context.Context, hash common.Hash) (*types.DetailTx, error) {
	tx, blockHash, blockNumber, index := rawdb.ReadTransaction(s.b.ChainDb(), hash)
	if tx == nil {
		return nil, nil
	}

	detailtxs := rawdb.ReadDetailTxs(s.b.ChainDb(), blockHash, blockNumber)
	if len(detailtxs) <= int(index) {
		return nil, nil
	}

	return detailtxs[index], nil
}

func (s *PublicBlockChainAPI) GetBadBlocks(ctx context.Context, fullTx bool) ([]map[string]interface{}, error) {
	blocks, err := s.b.GetBadBlocks(ctx)
	if len(blocks) != 0 {
		ret_block := make([]map[string]interface{}, len(blocks))

		for i, b := range blocks {
			ret_block[i] = s.rpcOutputBlock(s.b.ChainConfig().ChainID, b, true, fullTx)
		}

		return ret_block, nil
	}
	return nil, err
}

type CallArgs struct {
	ActionType types.ActionType `json:"actionType"`
	From       common.Name      `json:"from"`
	To         common.Name      `json:"to"`
	AssetID    uint64           `json:"assetId"`
	Gas        uint64           `json:"gas"`
	GasPrice   *big.Int         `json:"gasPrice"`
	Value      *big.Int         `json:"value"`
	Data       hexutil.Bytes    `json:"data"`
}

func (s *PublicBlockChainAPI) doCall(ctx context.Context, args CallArgs, blockNr rpc.BlockNumber, vmCfg vm.Config, timeout time.Duration) ([]byte, uint64, bool, error) {
	defer func(start time.Time) { log.Debug("Executing EVM call finished", "runtime", time.Since(start)) }(time.Now())

	state, header, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, 0, false, err
	}
	account, err := accountmanager.NewAccountManager(state)
	if err != nil {
		return nil, 0, false, err
	}

	gasPrice := args.GasPrice
	value := args.Value
	assetID := uint64(args.AssetID)
	gas := uint64(args.Gas)

	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	// Make sure the context is cancelled when the call has completed
	// this makes sure resources are cleaned up.
	defer cancel()

	// Get a new instance of the EVM.
	evm, vmError, err := s.b.GetEVM(ctx, account, state, args.From, assetID, gasPrice, header, vmCfg)
	if err != nil {
		return nil, 0, false, err
	}
	// Wait for the context to be done and cancel the evm. Even if the
	// EVM has finished, cancelling may be done (repeatedly)
	go func() {
		<-ctx.Done()
		evm.Cancel()
	}()

	// Setup the gas pool (also for unmetered requests)
	// and apply the message.
	gp := new(common.GasPool).AddGas(math.MaxUint64)
	action := types.NewAction(args.ActionType, args.From, args.To, 0, assetID, gas, value, args.Data)
	res, gas, failed, err, _ := processor.ApplyMessage(account, evm, action, gp, gasPrice, assetID, s.b.ChainConfig(), s.b.Engine())
	if err := vmError(); err != nil {
		return nil, 0, false, err
	}
	return res, gas, failed, err
}

// Call executes the given transaction on the state for the given block number.
// It doesn't make and changes in the state/blockchain and is useful to execute and retrieve values.
func (s *PublicBlockChainAPI) Call(ctx context.Context, args CallArgs, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	result, _, _, err := s.doCall(ctx, args, blockNr, vm.Config{}, 5*time.Second)
	return (hexutil.Bytes)(result), err
}

// EstimateGas returns an estimate of the amount of gas needed to execute the
// given transaction against the current pending block.
func (s *PublicBlockChainAPI) EstimateGas(ctx context.Context, args CallArgs) (hexutil.Uint64, error) {
	// Binary search the gas requirement, as it may be higher than the amount used
	var (
		lo  uint64 = params.ActionGas - 1
		hi  uint64
		cap uint64
	)
	if uint64(args.Gas) >= params.ActionGas {
		hi = uint64(args.Gas)
	} else {
		// Retrieve the current pending block to act as the gas ceiling
		block, err := s.b.BlockByNumber(ctx, rpc.PendingBlockNumber)
		if err != nil {
			return 0, err
		}
		hi = block.GasLimit()
	}
	cap = hi

	// Create a helper to check if a gas allowance results in an executable transaction
	executable := func(gas uint64) bool {
		args.Gas = gas

		_, _, failed, err := s.doCall(ctx, args, rpc.PendingBlockNumber, vm.Config{}, 0)
		if err != nil || failed {
			return false
		}
		return true
	}
	// Execute the binary search and hone in on an executable gas limit
	for lo+1 < hi {
		mid := (hi + lo) / 2
		if !executable(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	// Reject the transaction as invalid if it still fails at the highest allowance
	if hi == cap {
		if !executable(hi) {
			return 0, fmt.Errorf("gas required exceeds allowance or always failing transaction")
		}
	}
	return hexutil.Uint64(hi), nil
}

// GetChainConfig returns chain config.
func (s *PublicBlockChainAPI) GetChainConfig() map[string]interface{} {
	ret := map[string]interface{}{}
	g, err := s.b.BlockByNumber(context.Background(), 0)
	if err != nil {
		return ret
	}
	cfg := rawdb.ReadChainConfig(s.b.ChainDb(), g.Hash())
	bts, _ := json.Marshal(cfg)
	json.Unmarshal(bts, &ret)
	return ret
}

// GetGeneisisJson returns geneisis config.
func (s *PublicBlockChainAPI) GetGeneisis() map[string]interface{} {
	ret := map[string]interface{}{}
	g, err := s.b.BlockByNumber(context.Background(), 0)
	if err != nil {
		return ret
	}
	json.Unmarshal(g.Head.Extra, &ret)
	return ret
}