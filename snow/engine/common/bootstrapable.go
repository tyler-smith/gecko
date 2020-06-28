// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package common

import (
	"github.com/ava-labs/gecko/ids"
)

// Bootstrapable defines the functionality required to support bootstrapping
type Bootstrapable interface {
	// Returns the set of containerIDs that are accepted, but have no accepted
	// children.
	CurrentAcceptedFrontier() ids.Set

	// Returns the subset of containerIDs that are accepted by this chain.
	FilterAccepted(containerIDs ids.Set) (acceptedContainerIDs ids.Set)

	// Force the provided containers to be accepted. Only returns fatal errors
	// if they occur.
	ForceAccepted(acceptedContainerIDs ids.Set) error

	PersistEvents([][]byte) error
}
