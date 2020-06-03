// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"errors"
	"fmt"

	"github.com/ava-labs/gecko/database"
	"github.com/ava-labs/gecko/database/versiondb"
	"github.com/ava-labs/gecko/ids"
	"github.com/ava-labs/gecko/snow/validators"
	"github.com/ava-labs/gecko/utils/crypto"
	"github.com/ava-labs/gecko/utils/hashing"
	"github.com/ava-labs/gecko/vms/components/ava"
	"github.com/ava-labs/gecko/vms/components/verify"
)

var (
	errSigsNotSorted           = errors.New("control signatures not sorted")
	errWrongNumberOfSignatures = errors.New("wrong number of signatures")
	errDSValidatorSubset       = errors.New("all subnets must be a subset of the default subnet")
)

// UnsignedAddNonDefaultSubnetValidatorTx is an unsigned AddNonDefaultSubnetValidatorTx
type UnsignedAddNonDefaultSubnetValidatorTx struct {
	vm *VM

	// ID of this tx
	id ids.ID

	// Byte representation of the unsigned transaction
	unsignedBytes []byte

	// Byte representation of the signed transaction (ie with Creds and ControlSigs)
	bytes []byte

	// IDs of control keys
	controlIDs []ids.ShortID

	// The validator
	SubnetValidator `serialize:"true"`

	// ID of the network
	NetworkID uint32 `serialize:"true"`

	// Input UTXOs
	Ins []*ava.TransferableInput `serialize:"true"`

	// Output UTXOs
	Outs []*ava.TransferableOutput `serialize:"true"`
}

// UnsignedBytes returns the byte representation of this unsigned tx
func (tx *UnsignedAddNonDefaultSubnetValidatorTx) UnsignedBytes() []byte {
	return tx.unsignedBytes
}

// AddNonDefaultSubnetValidatorTx is a transaction that, if it is in a ProposeAddValidator block that
// is accepted and followed by a Commit block, adds a validator to the pending validator set of a subnet
// other than the default subnet.
// (That is, the validator in the tx will validate at some point in the future.)
// The transaction fee will be paid from the account whose ID is [Sigs[0].Address()]
type AddNonDefaultSubnetValidatorTx struct {
	UnsignedAddNonDefaultSubnetValidatorTx `serialize:"true"`

	// Credentials that authorize the inputs to spend the corresponding outputs
	Creds []verify.Verifiable `serialize:"true"`

	// When a subnet is created, it specifies a set of public keys ("control keys") such
	// that in order to add a validator to the subnet, a tx must be signed with
	// a certain threshold of those keys
	// Each element of ControlSigs is the signature of one of those keys
	ControlSigs [][crypto.SECP256K1RSigLen]byte `serialize:"true"`
}

// initialize [tx]
func (tx *AddNonDefaultSubnetValidatorTx) initialize(vm *VM) error {
	var err error
	tx.unsignedBytes, err = Codec.Marshal(interface{}(tx.UnsignedAddNonDefaultSubnetValidatorTx))
	if err != nil {
		fmt.Errorf("couldn't marshal UnsignedAddNonDefaultSubnetValidatorTx: %w", err)
	}
	tx.bytes, err = Codec.Marshal(tx) // byte representation of the signed transaction
	if err != nil {
		fmt.Errorf("couldn't marshal AddNonDefaultSubnetValidatorTx: %w", err)
	}
	tx.vm = vm
	tx.id = ids.NewID(hashing.ComputeHash256Array(tx.bytes))
	return nil
}

func (tx *AddNonDefaultSubnetValidatorTx) ID() ids.ID { return tx.id }

// SyntacticVerify return nil iff [tx] is valid
// If [tx] is valid, sets [tx.accountID]
// TODO: only verify once
func (tx *AddNonDefaultSubnetValidatorTx) SyntacticVerify() error {
	switch {
	case tx == nil:
		return errNilTx
	case tx.id.IsZero():
		return errInvalidID
	case tx.NetworkID != tx.vm.Ctx.NetworkID:
		return errWrongNetworkID
	case tx.NodeID.IsZero():
		return errInvalidID
	case tx.Subnet.IsZero():
		return errInvalidID
	case tx.Wght == 0: // Ensure the validator has some weight
		return errWeightTooSmall
	case !crypto.IsSortedAndUniqueSECP2561RSigs(tx.ControlSigs):
		return errSigsNotSorted
	}

	// Ensure staking length is not too short or long
	stakingDuration := tx.Duration()
	if stakingDuration < MinimumStakingDuration {
		return errStakeTooShort
	} else if stakingDuration > MaximumStakingDuration {
		return errStakeTooLong
	}

	// Byte representation of the unsigned transaction
	unsignedIntf := interface{}(&tx.UnsignedAddNonDefaultSubnetValidatorTx)
	unsignedBytes, err := Codec.Marshal(&unsignedIntf)
	if err != nil {
		return err
	}
	unsignedBytesHash := hashing.ComputeHash256(unsignedBytes)

	tx.controlIDs = make([]ids.ShortID, len(tx.ControlSigs))
	// recover control signatures
	for i, sig := range tx.ControlSigs {
		key, err := tx.vm.factory.RecoverHashPublicKey(unsignedBytesHash, sig[:])
		if err != nil {
			return err
		}
		tx.controlIDs[i] = key.Address()
	}

	if err := syntacticVerifySpend(tx.Ins, tx.Outs); err != nil {
		return err
	}

	return nil
}

// getDefaultSubnetStaker ...
func (h *EventHeap) getDefaultSubnetStaker(id ids.ShortID) (*AddDefaultSubnetValidatorTx, error) {
	for _, txIntf := range h.Txs {
		tx, ok := txIntf.(*AddDefaultSubnetValidatorTx)
		if !ok {
			continue
		}

		if id.Equals(tx.NodeID) {
			return tx, nil
		}
	}
	return nil, errors.New("couldn't find validator in the default subnet")
}

// SemanticVerify this transaction is valid.
func (tx *AddNonDefaultSubnetValidatorTx) SemanticVerify(db database.Database) (*versiondb.Database, *versiondb.Database, func(), func(), error) {
	// Ensure tx is syntactically valid
	if err := tx.SyntacticVerify(); err != nil {
		return nil, nil, nil, nil, err
	}

	// Get info about the subnet we're adding a validator to
	subnets, err := tx.vm.getSubnets(db)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var subnet *CreateSubnetTx
	for _, sn := range subnets {
		if sn.id.Equals(tx.SubnetID()) {
			subnet = sn
			break
		}
	}
	if subnet == nil {
		return nil, nil, nil, nil, fmt.Errorf("there is no subnet with ID %s", tx.SubnetID())
	}

	// Ensure the sigs on [tx] are valid
	if len(tx.ControlSigs) != int(subnet.Threshold) {
		return nil, nil, nil, nil, fmt.Errorf("expected tx to have %d control sigs but has %d", subnet.Threshold, len(tx.ControlSigs))
	}
	if !crypto.IsSortedAndUniqueSECP2561RSigs(tx.ControlSigs) {
		return nil, nil, nil, nil, errors.New("control signatures aren't sorted")
	}

	controlKeys := ids.ShortSet{}
	controlKeys.Add(subnet.ControlKeys...)
	for _, controlID := range tx.controlIDs {
		if !controlKeys.Contains(controlID) {
			return nil, nil, nil, nil, errors.New("tx has control signature from key not in subnet's ControlKeys")
		}
	}

	// Ensure that the period this validator validates the specified subnet is a subnet of the time they validate the default subnet
	// First, see if they're currently validating the default subnet
	currentDSValidators, err := tx.vm.getCurrentValidators(db, DefaultSubnetID)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("couldn't get current validators of default subnet: %v", err)
	}

	if dsValidator, err := currentDSValidators.getDefaultSubnetStaker(tx.NodeID); err == nil {
		if !tx.DurationValidator.BoundedBy(dsValidator.StartTime(), dsValidator.EndTime()) {
			return nil, nil, nil, nil,
				fmt.Errorf("time validating subnet [%v, %v] not subset of time validating default subnet [%v, %v]",
					tx.DurationValidator.StartTime(), tx.DurationValidator.EndTime(),
					dsValidator.StartTime(), dsValidator.EndTime())
		}
	} else {
		// They aren't currently validating the default subnet.
		// See if they will validate the default subnet in the future.
		pendingDSValidators, err := tx.vm.getPendingValidators(db, DefaultSubnetID)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("couldn't get pending validators of default subnet: %v", err)
		}
		dsValidator, err := pendingDSValidators.getDefaultSubnetStaker(tx.NodeID)
		if err != nil {
			return nil, nil, nil, nil,
				fmt.Errorf("validator would not be validating default subnet while validating non-default subnet")
		}
		if !tx.DurationValidator.BoundedBy(dsValidator.StartTime(), dsValidator.EndTime()) {
			return nil, nil, nil, nil,
				fmt.Errorf("time validating subnet [%v, %v] not subset of time validating default subnet [%v, %v]",
					tx.DurationValidator.StartTime(), tx.DurationValidator.EndTime(),
					dsValidator.StartTime(), dsValidator.EndTime())
		}
	}

	// Ensure the proposed validator starts after the current timestamp
	currentTimestamp, err := tx.vm.getTimestamp(db)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("couldn't get current timestamp: %v", err)
	}
	validatorStartTime := tx.StartTime()
	if !currentTimestamp.Before(validatorStartTime) {
		return nil, nil, nil, nil, fmt.Errorf("chain timestamp (%s) not before validator's start time (%s)",
			currentTimestamp,
			validatorStartTime)
	}

	// Get the account that is paying the transaction fee and, if the proposal is to add a validator
	// to the default subnet, providing the staked $AVA.
	// The ID of this account is the address associated with the public key that signed this tx
	accountID := tx.senderID
	account, err := tx.vm.getAccount(db, accountID)
	if err != nil {
		return nil, nil, nil, nil, errDBAccount
	}

	// The account if this block's proposal is committed and the validator is added
	// to the pending validator set. (Increase the account's nonce; decrease its balance.)
	newAccount, err := account.Remove(0, tx.Nonce) // Remove also removes the fee
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Ensure the proposed validator is not already a validator of the specified subnet
	currentEvents, err := tx.vm.getCurrentValidators(db, tx.Subnet)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("couldn't get current validators of subnet %s: %v", tx.Subnet, err)
	}
	currentValidators := validators.NewSet()
	currentValidators.Set(tx.vm.getValidators(currentEvents))
	if currentValidators.Contains(tx.NodeID) {
		return nil, nil, nil, nil, fmt.Errorf("validator with ID %s already in the current validator set for subnet with ID %s",
			tx.NodeID,
			tx.Subnet,
		)
	}

	// Ensure the proposed validator is not already slated to validate for the specified subnet
	pendingEvents, err := tx.vm.getPendingValidators(db, tx.Subnet)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("couldn't get pending validators of subnet %s: %v", tx.Subnet, err)
	}
	pendingValidators := validators.NewSet()
	pendingValidators.Set(tx.vm.getValidators(pendingEvents))
	if pendingValidators.Contains(tx.NodeID) {
		return nil, nil, nil, nil, fmt.Errorf("validator with ID %s already in the pending validator set for subnet with ID %s",
			tx.NodeID,
			tx.Subnet,
		)
	}

	pendingEvents.Add(tx) // add validator to set of pending validators

	// If this proposal is committed, update the pending validator set to include the validator,
	// update the validator's account by removing the staked $AVA
	onCommitDB := versiondb.New(db)
	if err := tx.vm.putPendingValidators(onCommitDB, pendingEvents, tx.Subnet); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("couldn't put current validators: %v", err)
	}
	if err := tx.vm.putAccount(onCommitDB, newAccount); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("couldn't put account: %v", err)
	}

	// If this proposal is aborted, chain state doesn't change
	onAbortDB := versiondb.New(db)

	return onCommitDB, onAbortDB, nil, nil, nil
}

// InitiallyPrefersCommit returns true if the proposed validators start time is
// after the current wall clock time,
func (tx *AddNonDefaultSubnetValidatorTx) InitiallyPrefersCommit() bool {
	return tx.StartTime().After(tx.vm.clock.Time())
}

func (vm *VM) newAddNonDefaultSubnetValidatorTx(
	nonce,
	weight,
	startTime,
	endTime uint64,
	nodeID ids.ShortID,
	subnetID ids.ID,
	networkID uint32,
	controlKeys []*crypto.PrivateKeySECP256K1R,
	payerKey *crypto.PrivateKeySECP256K1R,
) (*AddNonDefaultSubnetValidatorTx, error) {
	tx := &AddNonDefaultSubnetValidatorTx{
		UnsignedAddNonDefaultSubnetValidatorTx: UnsignedAddNonDefaultSubnetValidatorTx{
			SubnetValidator: SubnetValidator{
				DurationValidator: DurationValidator{
					Validator: Validator{
						NodeID: nodeID,
						Wght:   weight,
					},
					Start: startTime,
					End:   endTime,
				},
				Subnet: subnetID,
			},
			NetworkID: networkID,
		},
	}

	unsignedIntf := interface{}(&tx.UnsignedAddNonDefaultSubnetValidatorTx)
	unsignedBytes, err := Codec.Marshal(&unsignedIntf) // byte repr. of unsigned tx
	if err != nil {
		return nil, err
	}
	unsignedHash := hashing.ComputeHash256(unsignedBytes)

	// Sign this tx with each control key
	tx.ControlSigs = make([][crypto.SECP256K1RSigLen]byte, len(controlKeys))
	for i, key := range controlKeys {
		sig, err := key.SignHash(unsignedHash)
		if err != nil {
			return nil, err
		}
		// tx.ControlSigs[i] is type [65]byte but sig is type []byte
		// so we have to do the below
		copy(tx.ControlSigs[i][:], sig)
	}
	crypto.SortSECP2561RSigs(tx.ControlSigs)

	// Sign this tx with the key of the tx fee payer
	sig, err := payerKey.SignHash(unsignedHash)
	if err != nil {
		return nil, err
	}
	copy(tx.PayerSig[:], sig)

	return tx, tx.initialize(vm)
}
