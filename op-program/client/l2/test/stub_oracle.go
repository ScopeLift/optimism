package test

import (
	"encoding/binary"
	"testing"

	interopTypes "github.com/ethereum-optimism/optimism/op-program/client/interop/types"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
)

// Same as l2.StateOracle but need to use our own copy to avoid dependency loops
type stateOracle interface {
	NodeByHash(nodeHash common.Hash) []byte
	CodeByHash(codeHash common.Hash) []byte
}

type StubBlockOracle struct {
	t                *testing.T
	Blocks           map[common.Hash]*gethTypes.Block
	Outputs          map[common.Hash]eth.Output
	TransitionStates map[common.Hash]*interopTypes.TransitionState
	stateOracle
}

func NewStubOracle(t *testing.T) (*StubBlockOracle, *StubStateOracle) {
	stateOracle := NewStubStateOracle(t)
	blockOracle := StubBlockOracle{
		t:                t,
		Blocks:           make(map[common.Hash]*gethTypes.Block),
		Outputs:          make(map[common.Hash]eth.Output),
		TransitionStates: make(map[common.Hash]*interopTypes.TransitionState),
		stateOracle:      stateOracle,
	}
	return &blockOracle, stateOracle
}

func NewStubOracleWithBlocks(t *testing.T, chain []*gethTypes.Block, outputs []eth.Output, db ethdb.Database) *StubBlockOracle {
	blocks := make(map[common.Hash]*gethTypes.Block, len(chain))
	for _, block := range chain {
		blocks[block.Hash()] = block
	}
	o := make(map[common.Hash]eth.Output, len(outputs))
	for _, output := range outputs {
		o[common.Hash(eth.OutputRoot(output))] = output
	}
	return &StubBlockOracle{
		t:           t,
		Blocks:      blocks,
		Outputs:     o,
		stateOracle: &KvStateOracle{t: t, Source: db},
	}
}

func (o StubBlockOracle) BlockByHash(blockHash common.Hash) *gethTypes.Block {
	block, ok := o.Blocks[blockHash]
	if !ok {
		o.t.Fatalf("requested unknown block %s", blockHash)
	}
	return block
}

func (o StubBlockOracle) OutputByRoot(root common.Hash) eth.Output {
	output, ok := o.Outputs[root]
	if !ok {
		o.t.Fatalf("requested unknown output root %s", root)
	}
	return output
}
func (o StubBlockOracle) TransitionStateByRoot(root common.Hash) *interopTypes.TransitionState {
	output, ok := o.TransitionStates[root]
	if !ok {
		o.t.Fatalf("requested unknown transition state root %s", root)
	}
	return output
}

func (o StubBlockOracle) BlockDataByHash(agreedBlockHash, blockHash common.Hash, chainID uint64) *gethTypes.Block {
	block, ok := o.Blocks[blockHash]
	if !ok {
		o.t.Fatalf("requested unknown block %s", blockHash)
	}
	return block
}

// KvStateOracle loads data from a source ethdb.KeyValueStore
type KvStateOracle struct {
	t      *testing.T
	Source ethdb.KeyValueStore
}

func NewKvStateOracle(t *testing.T, db ethdb.KeyValueStore) *KvStateOracle {
	return &KvStateOracle{
		t:      t,
		Source: db,
	}
}

func (o *KvStateOracle) NodeByHash(nodeHash common.Hash) []byte {
	val, err := o.Source.Get(nodeHash.Bytes())
	if err != nil {
		o.t.Fatalf("error retrieving node %v: %v", nodeHash, err)
	}
	return val
}

func (o *KvStateOracle) CodeByHash(hash common.Hash) []byte {
	return rawdb.ReadCode(o.Source, hash)
}

func NewStubStateOracle(t *testing.T) *StubStateOracle {
	return &StubStateOracle{
		t:    t,
		Data: make(map[common.Hash][]byte),
		Code: make(map[common.Hash][]byte),
	}
}

// StubStateOracle is a StateOracle implementation that reads from simple maps
type StubStateOracle struct {
	t    *testing.T
	Data map[common.Hash][]byte
	Code map[common.Hash][]byte
}

func (o *StubStateOracle) NodeByHash(nodeHash common.Hash) []byte {
	data, ok := o.Data[nodeHash]
	if !ok {
		o.t.Fatalf("no value for node %v", nodeHash)
	}
	return data
}

func (o *StubStateOracle) CodeByHash(hash common.Hash) []byte {
	data, ok := o.Code[hash]
	if !ok {
		o.t.Fatalf("no value for code %v", hash)
	}
	return data
}

type StubPrecompileOracle struct {
	t       *testing.T
	Results map[common.Hash]PrecompileResult
	Calls   int
}

func NewStubPrecompileOracle(t *testing.T) *StubPrecompileOracle {
	return &StubPrecompileOracle{t: t, Results: make(map[common.Hash]PrecompileResult)}
}

type PrecompileResult struct {
	Result []byte
	Ok     bool
}

func (o *StubPrecompileOracle) Precompile(address common.Address, input []byte, requiredGas uint64) ([]byte, bool) {
	arg := append(address.Bytes(), binary.BigEndian.AppendUint64(nil, requiredGas)...)
	arg = append(arg, input...)
	result, ok := o.Results[crypto.Keccak256Hash(arg)]
	if !ok {
		o.t.Fatalf("no value for point evaluation %x required gas %v", input, requiredGas)
	}
	o.Calls++
	return result.Result, result.Ok
}
