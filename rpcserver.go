package main

import (
	"github.com/go-errors/errors"
	"github.com/the-lightning-land/balanced/balancer"
	"github.com/the-lightning-land/balanced/bdb"
	"github.com/the-lightning-land/balanced/rpc"
	"golang.org/x/net/context"
)

type rpcServerConfig struct {
	balancer *balancer.Balancer
	version  string
	commit   string
}

type rpcServer struct {
	balancer *balancer.Balancer
	version  string
	commit   string
}

// A compile time check to ensure that rpcServer fully implements the SweetServer gRPC service.
var _ rpc.BalanceServer = (*rpcServer)(nil)

func newRPCServer(config *rpcServerConfig) *rpcServer {
	return &rpcServer{
		balancer: config.balancer,
		version:  config.version,
		commit:   config.commit,
	}
}

func (s *rpcServer) GetInfo(ctx context.Context, req *rpc.GetInfoRequest) (*rpc.GetInfoResponse, error) {
	identityPubKey, _ := s.balancer.IdentityPubKey()

	return &rpc.GetInfoResponse{
		Version:        s.version,
		Commit:         s.commit,
		IdentityPubKey: string(identityPubKey),
	}, nil
}

func (s *rpcServer) Balance(ctx context.Context, req *rpc.BalanceRequest) (*rpc.BalanceResponse, error) {
	var err error
	rebalanced := false
	amtMsat := int64(500000000)

	if req.AmtMsat != 0 {
		amtMsat = int64(req.AmtMsat)
	}

	if req.FromChanId != 0 && req.ToChanId == 0 {
		rebalanced, err = s.balancer.Balance(amtMsat, bdb.ChanId(req.FromChanId), bdb.ChanId(req.ToChanId))
		if err != nil {
			return nil, errors.Errorf("Could not balance channel: %v", err)
		}
	} else if req.FromChanId != 0 && req.ToChanId != 0 {
		rebalanced, err = s.balancer.Balance(amtMsat, bdb.ChanId(req.FromChanId), bdb.ChanId(req.ToChanId))
		if err != nil {
			return nil, errors.Errorf("Could not balance channel: %v", err)
		}
	}

	return &rpc.BalanceResponse{
		Rebalanced: rebalanced,
	}, nil
}
