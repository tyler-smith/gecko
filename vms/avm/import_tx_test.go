// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package avm

import (
	"bytes"
	"testing"

	"github.com/ava-labs/gecko/chains/atomic"
	"github.com/ava-labs/gecko/database/memdb"
	"github.com/ava-labs/gecko/ids"
	"github.com/ava-labs/gecko/snow"
	"github.com/ava-labs/gecko/snow/engine/common"
	"github.com/ava-labs/gecko/utils/crypto"
	"github.com/ava-labs/gecko/utils/logging"
	"github.com/ava-labs/gecko/vms/components/ava"
	"github.com/ava-labs/gecko/vms/secp256k1fx"
)

func TestImportTxSerialization(t *testing.T) {
	expected := []byte{
		// txID:
		0x00, 0x00, 0x00, 0x03,
		// networkID:
		0x00, 0x00, 0x00, 0x02,
		// blockchainID:
		0xff, 0xff, 0xff, 0xff, 0xee, 0xee, 0xee, 0xee,
		0xdd, 0xdd, 0xdd, 0xdd, 0xcc, 0xcc, 0xcc, 0xcc,
		0xbb, 0xbb, 0xbb, 0xbb, 0xaa, 0xaa, 0xaa, 0xaa,
		0x99, 0x99, 0x99, 0x99, 0x88, 0x88, 0x88, 0x88,
		// number of base outs:
		0x00, 0x00, 0x00, 0x00,
		// number of base inputs:
		0x00, 0x00, 0x00, 0x00,
		// number of inputs:
		0x00, 0x00, 0x00, 0x01,
		// utxoID:
		0x0f, 0x2f, 0x4f, 0x6f, 0x8e, 0xae, 0xce, 0xee,
		0x0d, 0x2d, 0x4d, 0x6d, 0x8c, 0xac, 0xcc, 0xec,
		0x0b, 0x2b, 0x4b, 0x6b, 0x8a, 0xaa, 0xca, 0xea,
		0x09, 0x29, 0x49, 0x69, 0x88, 0xa8, 0xc8, 0xe8,
		// output index
		0x00, 0x00, 0x00, 0x00,
		// assetID:
		0x1f, 0x3f, 0x5f, 0x7f, 0x9e, 0xbe, 0xde, 0xfe,
		0x1d, 0x3d, 0x5d, 0x7d, 0x9c, 0xbc, 0xdc, 0xfc,
		0x1b, 0x3b, 0x5b, 0x7b, 0x9a, 0xba, 0xda, 0xfa,
		0x19, 0x39, 0x59, 0x79, 0x98, 0xb8, 0xd8, 0xf8,
		// input:
		// input ID:
		0x00, 0x00, 0x00, 0x05,
		// amount:
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0xe8,
		// num sig indices:
		0x00, 0x00, 0x00, 0x01,
		// sig index[0]:
		0x00, 0x00, 0x00, 0x00,
	}

	tx := &Tx{UnsignedTx: &ImportTx{
		BaseTx: BaseTx{
			NetID: 2,
			BCID: ids.NewID([32]byte{
				0xff, 0xff, 0xff, 0xff, 0xee, 0xee, 0xee, 0xee,
				0xdd, 0xdd, 0xdd, 0xdd, 0xcc, 0xcc, 0xcc, 0xcc,
				0xbb, 0xbb, 0xbb, 0xbb, 0xaa, 0xaa, 0xaa, 0xaa,
				0x99, 0x99, 0x99, 0x99, 0x88, 0x88, 0x88, 0x88,
			}),
		},
		Ins: []*ava.TransferableInput{{
			UTXOID: ava.UTXOID{TxID: ids.NewID([32]byte{
				0x0f, 0x2f, 0x4f, 0x6f, 0x8e, 0xae, 0xce, 0xee,
				0x0d, 0x2d, 0x4d, 0x6d, 0x8c, 0xac, 0xcc, 0xec,
				0x0b, 0x2b, 0x4b, 0x6b, 0x8a, 0xaa, 0xca, 0xea,
				0x09, 0x29, 0x49, 0x69, 0x88, 0xa8, 0xc8, 0xe8,
			})},
			Asset: ava.Asset{ID: ids.NewID([32]byte{
				0x1f, 0x3f, 0x5f, 0x7f, 0x9e, 0xbe, 0xde, 0xfe,
				0x1d, 0x3d, 0x5d, 0x7d, 0x9c, 0xbc, 0xdc, 0xfc,
				0x1b, 0x3b, 0x5b, 0x7b, 0x9a, 0xba, 0xda, 0xfa,
				0x19, 0x39, 0x59, 0x79, 0x98, 0xb8, 0xd8, 0xf8,
			})},
			In: &secp256k1fx.TransferInput{
				Amt:   1000,
				Input: secp256k1fx.Input{SigIndices: []uint32{0}},
			},
		}},
	}}

	c := setupCodec()
	b, err := c.Marshal(&tx.UnsignedTx)
	if err != nil {
		t.Fatal(err)
	}
	tx.Initialize(b)

	result := tx.Bytes()
	if !bytes.Equal(expected, result) {
		t.Fatalf("\nExpected: 0x%x\nResult:   0x%x", expected, result)
	}
}

// Test issuing an import transaction.
func TestIssueImportTx(t *testing.T) {
	genesisBytes := BuildGenesisTest(t)

	issuer := make(chan common.Message, 1)

	sm := &atomic.SharedMemory{}
	sm.Initialize(logging.NoLog{}, memdb.New())

	ctx := snow.DefaultContextTest()
	ctx.NetworkID = networkID
	ctx.ChainID = chainID
	ctx.SharedMemory = sm.NewBlockchainSharedMemory(chainID)

	genesisTx := GetFirstTxFromGenesisTest(genesisBytes, t)

	avaID := genesisTx.ID()
	platformID := ids.Empty.Prefix(0)

	ctx.Lock.Lock()
	vm := &VM{
		ava:      avaID,
		platform: platformID,
	}
	err := vm.Initialize(
		ctx,
		memdb.New(),
		genesisBytes,
		issuer,
		[]*common.Fx{{
			ID: ids.Empty,
			Fx: &secp256k1fx.Fx{},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	vm.batchTimeout = 0

	key := keys[0]

	utxoID := ava.UTXOID{
		TxID: ids.NewID([32]byte{
			0x0f, 0x2f, 0x4f, 0x6f, 0x8e, 0xae, 0xce, 0xee,
			0x0d, 0x2d, 0x4d, 0x6d, 0x8c, 0xac, 0xcc, 0xec,
			0x0b, 0x2b, 0x4b, 0x6b, 0x8a, 0xaa, 0xca, 0xea,
			0x09, 0x29, 0x49, 0x69, 0x88, 0xa8, 0xc8, 0xe8,
		}),
	}

	tx := &Tx{UnsignedTx: &ImportTx{
		BaseTx: BaseTx{
			NetID: networkID,
			BCID:  chainID,
		},
		Ins: []*ava.TransferableInput{{
			UTXOID: utxoID,
			Asset:  ava.Asset{ID: avaID},
			In: &secp256k1fx.TransferInput{
				Amt:   1000,
				Input: secp256k1fx.Input{SigIndices: []uint32{0}},
			},
		}},
	}}

	unsignedBytes, err := vm.codec.Marshal(&tx.UnsignedTx)
	if err != nil {
		t.Fatal(err)
	}

	sig, err := key.Sign(unsignedBytes)
	if err != nil {
		t.Fatal(err)
	}
	fixedSig := [crypto.SECP256K1RSigLen]byte{}
	copy(fixedSig[:], sig)

	tx.Creds = append(tx.Creds, &secp256k1fx.Credential{
		Sigs: [][crypto.SECP256K1RSigLen]byte{
			fixedSig,
		},
	})

	b, err := vm.codec.Marshal(tx)
	if err != nil {
		t.Fatal(err)
	}
	tx.Initialize(b)

	if _, err := vm.IssueTx(tx.Bytes(), nil); err == nil {
		t.Fatal(err)
	}

	// Provide the platform UTXO:

	smDB := vm.ctx.SharedMemory.GetDatabase(platformID)

	utxo := &ava.UTXO{
		UTXOID: utxoID,
		Asset:  ava.Asset{ID: avaID},
		Out: &secp256k1fx.TransferOutput{
			Amt: 1000,
			OutputOwners: secp256k1fx.OutputOwners{
				Threshold: 1,
				Addrs:     []ids.ShortID{key.PublicKey().Address()},
			},
		},
	}

	state := ava.NewPrefixedState(smDB, vm.codec)
	if err := state.FundPlatformUTXO(utxo); err != nil {
		t.Fatal(err)
	}

	vm.ctx.SharedMemory.ReleaseDatabase(platformID)

	if _, err := vm.IssueTx(tx.Bytes(), nil); err != nil {
		t.Fatalf("should have issued the transaction correctly but errored: %s", err)
	}

	ctx.Lock.Unlock()

	msg := <-issuer
	if msg != common.PendingTxs {
		t.Fatalf("Wrong message")
	}

	txs := vm.PendingTxs()
	if len(txs) != 1 {
		t.Fatalf("Should have returned %d tx(s)", 1)
	}

	parsedTx := txs[0]
	parsedTx.Accept()

	smDB = vm.ctx.SharedMemory.GetDatabase(platformID)
	defer vm.ctx.SharedMemory.ReleaseDatabase(platformID)

	state = ava.NewPrefixedState(smDB, vm.codec)
	if _, err := state.PlatformUTXO(utxoID.InputID()); err == nil {
		t.Fatalf("shouldn't have been able to read the utxo")
	}
}

// Test force accepting an import transaction.
func TestForceAcceptImportTx(t *testing.T) {
	genesisBytes := BuildGenesisTest(t)

	issuer := make(chan common.Message, 1)

	sm := &atomic.SharedMemory{}
	sm.Initialize(logging.NoLog{}, memdb.New())

	ctx := snow.DefaultContextTest()
	ctx.NetworkID = networkID
	ctx.ChainID = chainID
	ctx.SharedMemory = sm.NewBlockchainSharedMemory(chainID)

	platformID := ids.Empty.Prefix(0)

	ctx.Lock.Lock()
	defer ctx.Lock.Unlock()

	vm := &VM{platform: platformID}
	err := vm.Initialize(
		ctx,
		memdb.New(),
		genesisBytes,
		issuer,
		[]*common.Fx{{
			ID: ids.Empty,
			Fx: &secp256k1fx.Fx{},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	vm.batchTimeout = 0

	key := keys[0]

	genesisTx := GetFirstTxFromGenesisTest(genesisBytes, t)

	utxoID := ava.UTXOID{
		TxID: ids.NewID([32]byte{
			0x0f, 0x2f, 0x4f, 0x6f, 0x8e, 0xae, 0xce, 0xee,
			0x0d, 0x2d, 0x4d, 0x6d, 0x8c, 0xac, 0xcc, 0xec,
			0x0b, 0x2b, 0x4b, 0x6b, 0x8a, 0xaa, 0xca, 0xea,
			0x09, 0x29, 0x49, 0x69, 0x88, 0xa8, 0xc8, 0xe8,
		}),
	}

	tx := &Tx{UnsignedTx: &ImportTx{
		BaseTx: BaseTx{
			NetID: networkID,
			BCID:  chainID,
		},
		Ins: []*ava.TransferableInput{{
			UTXOID: utxoID,
			Asset:  ava.Asset{ID: genesisTx.ID()},
			In: &secp256k1fx.TransferInput{
				Amt:   1000,
				Input: secp256k1fx.Input{SigIndices: []uint32{0}},
			},
		}},
	}}

	unsignedBytes, err := vm.codec.Marshal(&tx.UnsignedTx)
	if err != nil {
		t.Fatal(err)
	}

	sig, err := key.Sign(unsignedBytes)
	if err != nil {
		t.Fatal(err)
	}
	fixedSig := [crypto.SECP256K1RSigLen]byte{}
	copy(fixedSig[:], sig)

	tx.Creds = append(tx.Creds, &secp256k1fx.Credential{
		Sigs: [][crypto.SECP256K1RSigLen]byte{
			fixedSig,
		},
	})

	b, err := vm.codec.Marshal(tx)
	if err != nil {
		t.Fatal(err)
	}
	tx.Initialize(b)

	parsedTx, err := vm.ParseTx(tx.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	if err := parsedTx.Verify(); err == nil {
		t.Fatalf("Should have failed verification")
	}

	parsedTx.Accept()

	smDB := vm.ctx.SharedMemory.GetDatabase(platformID)
	defer vm.ctx.SharedMemory.ReleaseDatabase(platformID)

	state := ava.NewPrefixedState(smDB, vm.codec)
	utxoSource := utxoID.InputID()
	if _, err := state.PlatformUTXO(utxoSource); err == nil {
		t.Fatalf("shouldn't have been able to read the utxo")
	}
}
