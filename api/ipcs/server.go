// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ipcs

import (
	"fmt"
	"net/http"

	"nanomsg.org/go/mangos/v2/protocol/pub"

	_ "nanomsg.org/go/mangos/v2/transport/ipc" // registers the IPC transport

	"github.com/gorilla/rpc/v2"

	"github.com/ava-labs/gecko/api"
	"github.com/ava-labs/gecko/chains"
	"github.com/ava-labs/gecko/ids"
	"github.com/ava-labs/gecko/snow/engine/common"
	"github.com/ava-labs/gecko/snow/triggers"
	"github.com/ava-labs/gecko/utils/json"
	"github.com/ava-labs/gecko/utils/logging"
	"github.com/ava-labs/gecko/utils/wrappers"
)

const baseURL = "ipc:///tmp/"

// IPCs maintains the IPCs
type IPCs struct {
	log          logging.Logger
	chainManager chains.Manager
	httpServer   *api.Server
	events       *triggers.EventDispatcher
	chains       map[[32]byte]*ChainIPC
}

// NewService returns a new IPCs API service
func NewService(log logging.Logger, chainManager chains.Manager, events *triggers.EventDispatcher, defaultChainIDs []string, httpServer *api.Server) (*common.HTTPHandler, error) {
	ipcs := &IPCs{
		log:          log,
		chainManager: chainManager,
		httpServer:   httpServer,
		events:       events,
		chains:       map[[32]byte]*ChainIPC{},
	}

	for _, chainID := range defaultChainIDs {
		id, err := ids.FromString(chainID)
		if err != nil {
			return nil, err
		}
		if _, err := ipcs.publish(id); err != nil {
			return nil, err
		}
	}

	newServer := rpc.NewServer()
	codec := json.NewCodec()
	newServer.RegisterCodec(codec, "application/json")
	newServer.RegisterCodec(codec, "application/json;charset=UTF-8")
	newServer.RegisterService(ipcs, "ipcs")
	return &common.HTTPHandler{Handler: newServer}, nil
}

// PublishBlockchainArgs are the arguments for calling PublishBlockchain
type PublishBlockchainArgs struct {
	BlockchainID string `json:"blockchainID"`
}

// PublishBlockchainReply are the results from calling PublishBlockchain
type PublishBlockchainReply struct {
	URL string `json:"url"`
}

// PublishBlockchain publishes the finalized accepted transactions from the blockchainID over the IPC
func (ipc *IPCs) PublishBlockchain(r *http.Request, args *PublishBlockchainArgs, reply *PublishBlockchainReply) error {
	ipc.log.Info("IPCs: PublishBlockchain called with BlockchainID: %s", args.BlockchainID)
	chainID, err := ipc.chainManager.Lookup(args.BlockchainID)
	if err != nil {
		ipc.log.Error("unknown blockchainID: %s", err)
		return err
	}
	cipc, err := ipc.publish(chainID)
	if err != nil {
		ipc.log.Error("couldn't publish blockchainID: %s", err)
		return err
	}

	reply.URL = cipc.url

	return nil
}

// UnpublishBlockchainArgs are the arguments for calling UnpublishBlockchain
type UnpublishBlockchainArgs struct {
	BlockchainID string `json:"blockchainID"`
}

// UnpublishBlockchainReply are the results from calling UnpublishBlockchain
type UnpublishBlockchainReply struct {
	Success bool `json:"success"`
}

// UnpublishBlockchain closes publishing of a blockchainID
func (ipc *IPCs) UnpublishBlockchain(r *http.Request, args *UnpublishBlockchainArgs, reply *UnpublishBlockchainReply) error {
	ipc.log.Info("IPCs: UnpublishBlockchain called with BlockchainID: %s", args.BlockchainID)
	chainID, err := ipc.chainManager.Lookup(args.BlockchainID)
	if err != nil {
		ipc.log.Error("unknown blockchainID %s: %s", args.BlockchainID, err)
		return err
	}

	chainIDKey := chainID.Key()

	chain, ok := ipc.chains[chainIDKey]
	if !ok {
		return fmt.Errorf("blockchainID not publishing: %s", chainID)
	}

	errs := wrappers.Errs{}
	errs.Add(
		chain.Stop(),
		ipc.events.DeregisterChain(chainID, "ipc"),
	)
	delete(ipc.chains, chainIDKey)

	reply.Success = true
	return errs.Err
}

func (ipc *IPCs) publish(chainID ids.ID) (*ChainIPC, error) {
	chainIDKey := chainID.Key()
	chainIDStr := chainID.String()

	url := baseURL + chainID.String() + ".ipc"

	if cipc, ok := ipc.chains[chainIDKey]; ok {
		ipc.log.Info("returning existing blockchainID %s", chainIDStr)
		return cipc, nil
	}

	sock, err := pub.NewSocket()
	if err != nil {
		ipc.log.Error("can't get new pub socket: %s", err)
		return nil, err
	}

	if err = sock.Listen(url); err != nil {
		ipc.log.Error("can't listen on pub socket: %s", err)
		sock.Close()
		return nil, err
	}

	chainIPC := &ChainIPC{
		url:    url,
		log:    ipc.log,
		socket: sock,
	}
	if err := ipc.events.RegisterChain(chainID, "ipc", chainIPC); err != nil {
		ipc.log.Error("couldn't register event: %s", err)
		sock.Close()
		return nil, err
	}

	ipc.chains[chainIDKey] = chainIPC
	return chainIPC, nil
}
