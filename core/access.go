package core

import (
	"fmt"
)

// Access is the instance on which lazy accessors (core/chain_util)
// allowing ODR should be called
// GetBody, GetBlock, GetBlockRLP all fall back to only GetBodyRLP
type Access struct {
	GetBodyRLP       func(db ethdb.Database, hash common.Hash) rlp.RawValue
	GetBlockReceipts func(db ethdb.Database, hash common.Hash) types.Receipts
	// GetNodeData?
	// GetTrieEntry?
}

// Access type using no ODR just local DB simply sets GetBlockRLP, GetBlockReceipts
// functions to the ones in core/chain_util
func NewLocalAccess() *Access {
	return &Access{
		GetBodyRLP:       GetBodyRLP,
		GetBlockReceipts: GetBlockReceipts,
	}
}

// this creates an Access instance that uses the these accessor functions
// only as a fallback if local access gives no result
// in case the fallback call is needed and succeeds, the result is written
// to local Db
// This simplifies all the DbGet/DbPut logic
// This fallback logic can be directly implemented in core/odr.Odr functions
// and does not need to be implemented here
func NewFallBackAccess(
	getBodyRLP func(db ethdb.Database, hash common.Hash) rlp.RawValue,
	getRawBlockReceipts func(db ethdb.Database, hash common.Hash) []byte,
) *Access {
	return &Access{
		GetBodyRLP: func(db ethdb.Database, hash common.Hash) rlp.RawValue {
			data := GetBodyRLP(db, hash)
			if data == nil {
				data := getBodyRLP(db, hash)
				if data != nil {
					WriteBodyRLP(db, hash, data)
				}
			}
			return data
		},
		GetRawBlockReceipts: func(db ethdb.Database, hash common.Hash) [] {
			data := GetBlockReceipts(db, hash)
			if len(data) == 0 {
				data := getBlockReceipts(db, hash)
				if len(data) > 0 {
					WriteBlockReceipts(db, hash, data)
					WriteReceipts(db, hash, data)
				}
			}
			return data
		},
	}
}

// GetBody retrieves the block body (transactons, uncles) corresponding to the
// hash, nil if none found.
func (self *Access) GetBody(db ethdb.Database, hash common.Hash) *types.Body {
	data := GetBodyRLP(db, hash)
	if len(data) == 0 {
		return nil
	}
	body := new(types.Body)
	if err := rlp.Decode(bytes.NewReader(data), body); err != nil {
		glog.V(logger.Error).Infof("invalid block body RLP for hash %x: %v", hash, err)
		return nil
	}
	return body
}

func (self *Access) GetRawBlockReceipts(db ethdb.Database, hash common.Hash) []byte {
	data, _ := db.Get(append(blockReceiptsPrefix, hash[:]...))
	return data
}

// GetBlock retrieves an entire block corresponding to the hash, assembling it
// back from the stored header and body.
func (self *Access) GetBlock(db ethdb.Database, hash common.Hash) *types.Block {
	// Retrieve the block header and body contents
	header := GetHeader(db, hash)
	if header == nil {
		return nil
	}
	body := GetBody(db, hash)
	if body == nil {
		return nil
	}
	// Reassemble the block and return
	return types.NewBlockWithHeader(header).WithBody(body.Transactions, body.Uncles)
}

// GetBlockReceipts retrieves the receipts generated by the transactions included
// in a block given by its hash.
func (self *Access) GetBlockReceipts(db ethdb.Database, hash common.Hash) types.Receipts {
	receipts := self.GetRawBlockReceipts(db, hash)
	if receipts == nil {
		return nil
	}
	storageReceipts := []*types.ReceiptForStorage{}
	if err := rlp.DecodeBytes(data, &storageReceipts); err != nil {
		glog.V(logger.Error).Infof("invalid receipt array RLP for hash %x: %v", hash, err)
		return nil
	}
	receipts := make(types.Receipts, len(storageReceipts))
	for i, receipt := range storageReceipts {
		receipts[i] = (*types.Receipt)(receipt)
	}
	return receipts
}
