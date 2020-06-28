// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package common

import (
	"bufio"
	"encoding/hex"
	stdmath "math"
	"os"
	"path"
	"time"

	"github.com/ava-labs/gecko/ids"
	"github.com/ava-labs/gecko/utils/hashing"
	"github.com/ava-labs/gecko/utils/math"
)

const (
	// MaxContainersPerMultiPut is the maximum number of containers that can be sent in a MultiPut
	MaxContainersPerMultiPut = 2000

	// StatusUpdateFrequency ... bootstrapper logs "processed X blocks/vertices" every [statusUpdateFrequency] blocks/vertices
	StatusUpdateFrequency = 2500

	// MaxOutstandingRequests is the maximum number of GetAncestors sent but not responsded to/failed
	MaxOutstandingRequests = 8
)

var (
	// MaxTimeFetchingAncestors is the maximum amount of time to spend fetching vertices during a call to GetAncestors
	MaxTimeFetchingAncestors = 50 * time.Millisecond
)

// Bootstrapper implements the Engine interface.
type Bootstrapper struct {
	Config

	// IDs of validators we have requested the accepted frontier from but haven't
	// received a reply from
	pendingAcceptedFrontier ids.ShortSet
	acceptedFrontier        ids.Set

	pendingAccepted ids.ShortSet
	acceptedVotes   map[[32]byte]uint64

	RequestID uint32
}

// Initialize implements the Engine interface.
func (b *Bootstrapper) Initialize(config Config) {
	if config.Context.RecoveryDir != "" {
		b.InitializeFromFile(config)
		return
	}

	b.Config = config

	for _, vdr := range b.Beacons.List() {
		vdrID := vdr.ID()
		b.pendingAcceptedFrontier.Add(vdrID)
		b.pendingAccepted.Add(vdrID)
	}

	b.acceptedVotes = make(map[[32]byte]uint64)
}

func (b *Bootstrapper) InitializeFromFile(config Config) {
	b.Config = config

	filepath := path.Join(config.Context.RecoveryDir, config.Context.ChainID.String()+".txt")

	b.Context.Log.Info("Bootstrapping from recovery file: %s", filepath)

	file, err := os.Open(filepath)
	if err != nil {
		b.Context.Log.Fatal("Failed to open file: %s", err.Error())
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	containers := [][]byte{}
	for scanner.Scan() {
		bytes, err := hex.DecodeString(scanner.Text())
		if err != nil {
			b.Context.Log.Fatal("Failed to decode consensus object: %s", err.Error())
			return
		}
		containers = append(containers, bytes)
	}

	if err = b.Bootstrapable.PersistEvents(containers); err != nil {
		b.Context.Log.Fatal("Failed to persist consensus events: %s", err.Error())
		return
	}

	if err := scanner.Err(); err != nil {
		b.Context.Log.Fatal("Failed to scan recovery file: %s", err.Error())
		return
	}

	tipSet := ids.Set{}
	tipSet.Add(ids.NewID(hashing.ComputeHash256Array(containers[len(containers)-1])))
	if err = b.Bootstrapable.ForceAccepted(tipSet); err != nil {
		b.Context.Log.Fatal("Failed force accepted: %s", err.Error())
		return
	}
}

// Startup implements the Engine interface.
func (b *Bootstrapper) Startup() error {
	if b.pendingAcceptedFrontier.Len() == 0 {
		b.Context.Log.Info("Bootstrapping skipped due to no provided bootstraps")
		return b.Bootstrapable.ForceAccepted(ids.Set{})
	}

	// Ask each of the bootstrap validators to send their accepted frontier
	vdrs := ids.ShortSet{}
	vdrs.Union(b.pendingAcceptedFrontier)

	b.RequestID++
	b.Sender.GetAcceptedFrontier(vdrs, b.RequestID)
	return nil
}

// GetAcceptedFrontier implements the Engine interface.
func (b *Bootstrapper) GetAcceptedFrontier(validatorID ids.ShortID, requestID uint32) error {
	b.Sender.AcceptedFrontier(validatorID, requestID, b.Bootstrapable.CurrentAcceptedFrontier())
	return nil
}

// GetAcceptedFrontierFailed implements the Engine interface.
func (b *Bootstrapper) GetAcceptedFrontierFailed(validatorID ids.ShortID, requestID uint32) error {
	// If we can't get a response from [validatorID], act as though they said their accepted frontier is empty
	b.AcceptedFrontier(validatorID, requestID, ids.Set{})
	return nil
}

// AcceptedFrontier implements the Engine interface.
func (b *Bootstrapper) AcceptedFrontier(validatorID ids.ShortID, requestID uint32, containerIDs ids.Set) error {
	if !b.pendingAcceptedFrontier.Contains(validatorID) {
		b.Context.Log.Debug("Received an AcceptedFrontier message from %s unexpectedly", validatorID)
		return nil
	}
	// Mark that we received a response from [validatorID]
	b.pendingAcceptedFrontier.Remove(validatorID)

	// Union the reported accepted frontier from [validatorID] with the accepted frontier we got from others
	b.acceptedFrontier.Union(containerIDs)

	// We've received the accepted frontier from every bootstrap validator
	// Ask each bootstrap validator to filter the list of containers that we were
	// told are on the accepted frontier such that the list only contains containers
	// they think are accepted
	if b.pendingAcceptedFrontier.Len() == 0 {
		vdrs := ids.ShortSet{}
		vdrs.Union(b.pendingAccepted)

		b.RequestID++
		b.Sender.GetAccepted(vdrs, b.RequestID, b.acceptedFrontier)
	}
	return nil
}

// GetAccepted implements the Engine interface.
func (b *Bootstrapper) GetAccepted(validatorID ids.ShortID, requestID uint32, containerIDs ids.Set) error {
	b.Sender.Accepted(validatorID, requestID, b.Bootstrapable.FilterAccepted(containerIDs))
	return nil
}

// GetAcceptedFailed implements the Engine interface.
func (b *Bootstrapper) GetAcceptedFailed(validatorID ids.ShortID, requestID uint32) error {
	// If we can't get a response from [validatorID], act as though they said
	// that they think none of the containers we sent them in GetAccepted are accepted
	return b.Accepted(validatorID, requestID, ids.Set{})
}

// Accepted implements the Engine interface.
func (b *Bootstrapper) Accepted(validatorID ids.ShortID, requestID uint32, containerIDs ids.Set) error {
	if !b.pendingAccepted.Contains(validatorID) {
		b.Context.Log.Debug("Received an Accepted message from %s unexpectedly", validatorID)
		return nil
	}
	// Mark that we received a response from [validatorID]
	b.pendingAccepted.Remove(validatorID)

	weight := uint64(0)
	if vdr, ok := b.Beacons.Get(validatorID); ok {
		weight = vdr.Weight()
	}

	for _, containerID := range containerIDs.List() {
		key := containerID.Key()
		previousWeight := b.acceptedVotes[key]
		newWeight, err := math.Add64(weight, previousWeight)
		if err != nil {
			newWeight = stdmath.MaxUint64
		}
		b.acceptedVotes[key] = newWeight
	}

	if b.pendingAccepted.Len() != 0 {
		return nil
	}

	// We've received the filtered accepted frontier from every bootstrap validator
	// Accept all containers that have a sufficient weight behind them
	accepted := ids.Set{}
	for key, weight := range b.acceptedVotes {
		if weight >= b.Config.Alpha {
			accepted.Add(ids.NewID(key))
		}
	}

	if size := accepted.Len(); size == 0 && b.Config.Beacons.Len() > 0 {
		b.Context.Log.Info("Bootstrapping finished with no accepted frontier. This is likely a result of failing to be able to connect to the specified bootstraps, or no transactions have been issued on this chain yet")
	} else {
		b.Context.Log.Info("Bootstrapping started syncing with %d vertices in the accepted frontier", size)
	}

	return b.Bootstrapable.ForceAccepted(accepted)
}
