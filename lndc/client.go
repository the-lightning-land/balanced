package lndc

import (
	"context"
	"crypto/x509"
	"encoding/hex"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"github.com/the-lightning-land/balanced/bdb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"io/ioutil"
	"regexp"
)

type Client struct {
	client         lnrpc.LightningClient
	context        context.Context
	identityPubKey bdb.PubKey
	blockHeight    bdb.BlockHeight
}

type Config struct {
	TlsCertPath  string
	RpcServer    string
	MacaroonPath string
}

func NewClient(config *Config) (*Client, error) {
	cert, err := makeTlsCertFromPath(config.TlsCertPath)
	if err != nil {
		return nil, errors.Errorf("Could not make TLS cert: %v", err)
	}

	creds := credentials.NewClientTLSFromCert(cert, "")

	conn, err := grpc.Dial(config.RpcServer, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, errors.Errorf("Could not connect to lightning node: %v", err)
	}

	client := lnrpc.NewLightningClient(conn)

	macaroon, err := makeMacaroonFromPath(config.MacaroonPath)
	if err != nil {
		return nil, errors.Errorf("Could not make macaroon: %v", err)
	}

	ctx := context.Background()
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("macaroon", macaroon))

	return &Client{
		client:  client,
		context: ctx,
	}, nil
}

func (client *Client) Start() error {
	return nil
}

func (client *Client) Stop() error {
	return nil
}

// Graph returns the local Lightning network graph consisting of a node and edge map
func (client *Client) Graph() (*bdb.Graph, error) {
	graph := &bdb.Graph{}

	channelGraph, err := client.client.DescribeGraph(client.context, &lnrpc.ChannelGraphRequest{
		IncludeUnannounced: true,
	}, grpc.MaxCallRecvMsgSize(50*1024*1024))
	if err != nil {
		return nil, errors.Errorf("Could not get channel graph: %v", err)
	}

	nodeMap := make(bdb.NodeMap)
	for _, node := range channelGraph.Nodes {
		nodeMap[bdb.PubKey(node.PubKey)] = &bdb.Node{
			PubKey: bdb.PubKey(node.PubKey),
			Alias:  node.Alias,
		}
	}
	graph.Nodes = nodeMap

	edgeMap := make(bdb.EdgeMap)
	for _, edge := range channelGraph.Edges {
		if edge.Node2Policy != nil {
			edgeMap[bdb.EdgeId(edge.ChannelId)] = &bdb.Edge{
				Id:         bdb.EdgeId(edge.ChannelId),
				FromNode:   bdb.PubKey(edge.Node1Pub),
				ToNode:     bdb.PubKey(edge.Node2Pub),
				ChanId:     bdb.ChanId(edge.ChannelId),
				Capacity:   edge.Capacity,
				ChanPoint:  edge.ChanPoint,
				LastUpdate: edge.LastUpdate,
				Policy: &bdb.RoutingPolicy{
					FeeRateMilliMsat: edge.Node2Policy.FeeRateMilliMsat,
					FeeBaseMsat:      edge.Node2Policy.FeeBaseMsat,
					TimeLockDelta:    edge.Node2Policy.TimeLockDelta,
					Disabled:         edge.Node2Policy.Disabled,
					MaxHtlcMsat:      edge.Node2Policy.MaxHtlcMsat,
					MinHtlc:          edge.Node2Policy.MinHtlc,
				},
			}
		}

		if edge.Node1Policy != nil {
			edgeMap[bdb.EdgeId(edge.ChannelId)+1] = &bdb.Edge{
				Id:         bdb.EdgeId(edge.ChannelId),
				FromNode:   bdb.PubKey(edge.Node2Pub),
				ToNode:     bdb.PubKey(edge.Node1Pub),
				ChanId:     bdb.ChanId(edge.ChannelId),
				Capacity:   edge.Capacity,
				ChanPoint:  edge.ChanPoint,
				LastUpdate: edge.LastUpdate,
				Policy: &bdb.RoutingPolicy{
					FeeRateMilliMsat: edge.Node1Policy.FeeRateMilliMsat,
					FeeBaseMsat:      edge.Node1Policy.FeeBaseMsat,
					TimeLockDelta:    edge.Node1Policy.TimeLockDelta,
					Disabled:         edge.Node1Policy.Disabled,
					MaxHtlcMsat:      edge.Node1Policy.MaxHtlcMsat,
					MinHtlc:          edge.Node1Policy.MinHtlc,
				},
			}
		}
	}
	graph.Edges = edgeMap

	return graph, nil
}

func (client *Client) BlockHeight() (bdb.BlockHeight, error) {
	// Once saved, we can assume that the block height stays for a while
	if client.blockHeight != 0 {
		return client.blockHeight, nil
	}

	info, err := client.client.GetInfo(client.context, &lnrpc.GetInfoRequest{})
	if err != nil {
		return 0, errors.Errorf("Could not get info: %v", err)
	}

	client.blockHeight = bdb.BlockHeight(info.BlockHeight)

	return bdb.BlockHeight(info.BlockHeight), nil
}

func (client *Client) IdentitiyPubKey() (bdb.PubKey, error) {
	// Once saved, we can assume that the identity pubkey always stays the same
	if client.identityPubKey != "" {
		return client.identityPubKey, nil
	}

	info, err := client.client.GetInfo(client.context, &lnrpc.GetInfoRequest{})
	if err != nil {
		return "", errors.Errorf("Could not get info: %v", err)
	}

	client.identityPubKey = bdb.PubKey(info.IdentityPubkey)

	return bdb.PubKey(info.IdentityPubkey), nil
}

func (client *Client) Channels() (bdb.ChannelMap, error) {
	channelList, err := client.client.ListChannels(client.context, &lnrpc.ListChannelsRequest{
		ActiveOnly: true,
	})
	if err != nil {
		return nil, errors.Errorf("Could not list channels: %v", err)
	}

	channels := make(bdb.ChannelMap)
	for _, channel := range channelList.Channels {
		channels[bdb.ChanId(channel.ChanId)] = &bdb.Channel{
			ChanId:       bdb.ChanId(channel.ChanId),
			Capacity:     uint64(channel.Capacity),
			LocalBalance: uint64(channel.LocalBalance),
			ToNode:       bdb.PubKey(channel.RemotePubkey),
			Active:       channel.Active,
		}
	}

	return channels, nil
}

func (client *Client) AddInvoice(amt int64) (*bdb.Invoice, error) {
	addInvoice, err := client.client.AddInvoice(client.context, &lnrpc.Invoice{
		CltvExpiry: 9,
		Value:      int64(0.001 * float32(amt)),
	})
	if err != nil {
		return nil, errors.Errorf("Could not add invoice: %v", err)
	}

	decodedInvoice, err := client.client.DecodePayReq(client.context, &lnrpc.PayReqString{
		PayReq: addInvoice.PaymentRequest,
	})
	if err != nil {
		return nil, errors.Errorf("Could not decode invoice: %v", err)
	}

	return &bdb.Invoice{
		Expiry:          decodedInvoice.Expiry,
		NumSatoshis:     decodedInvoice.NumSatoshis,
		PaymentHash:     decodedInvoice.PaymentHash,
		CltvExpiry:      decodedInvoice.CltvExpiry,
		Description:     decodedInvoice.Description,
		DescriptionHash: decodedInvoice.DescriptionHash,
		Destination:     decodedInvoice.Destination,
		FallbackAddr:    decodedInvoice.FallbackAddr,
		Timestamp:       decodedInvoice.Timestamp,
	}, nil
}

func (client *Client) SendToRoute(req *lnrpc.SendToRouteRequest) (*bdb.Payment, error) {
	sendResult, err := client.client.SendToRouteSync(client.context, req)
	if err != nil {
		return nil, errors.Errorf("Could not send to route: %v", err)
	}

	if sendResult.PaymentError != "" {
		err := mapPaymentErrorToTypedError(sendResult.PaymentError)

		// Extend the temporary channel failure error with amount that couldn't be forwarded
		switch err := err.(type) {
		case bdb.TemporaryChannelFailureError:
			for _, hop := range req.Route.Hops {
				if hop.ChanId == uint64(err.ChanId) {
					err.AmtMsat = hop.AmtToForwardMsat
				}
			}
		}

		return nil, err
	}

	return &bdb.Payment{
		PaymentHash:     hex.EncodeToString(sendResult.PaymentHash),
		PaymentPreimage: hex.EncodeToString(sendResult.PaymentPreimage),
		FeesMsat:        sendResult.PaymentRoute.TotalFeesMsat,
		AmtMsat:         sendResult.PaymentRoute.TotalAmtMsat,
	}, nil
}

func mapPaymentErrorToTypedError(paymentError string) error {
	var temporaryChannelFailureRegex = regexp.MustCompile(`(?ms)TemporaryChannelFailure.*\b(?P<ShortChanId>\d+:\d+:\d+)\b`)

	temporaryChannelFailure := temporaryChannelFailureRegex.FindStringSubmatch(paymentError)
	if len(temporaryChannelFailure) > 0 {
		// It's a temporary channel failure
		shortChanId, _ := bdb.NewShortChanIdFromString(temporaryChannelFailure[1])

		return bdb.TemporaryChannelFailureError{
			ChanId: bdb.ChanId(shortChanId.ToUint64()),
		}
	}

	return errors.Errorf("Could not send payment: %v", paymentError)
}

func makeTlsCertFromPath(path string) (*x509.CertPool, error) {
	certBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Errorf("Could not read tls cert %v", path)
	}

	cert := x509.NewCertPool()
	fullCertBytes := append([]byte("-----BEGIN CERTIFICATE-----\n"), certBytes...)
	fullCertBytes = append(fullCertBytes, []byte("\n-----END CERTIFICATE-----")...)
	if ok := cert.AppendCertsFromPEM(fullCertBytes); !ok {
		return nil, errors.New("Could not parse tls cert.")
	}

	return cert, nil
}

func makeMacaroonFromPath(path string) (string, error) {
	macaroonBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return "", errors.Errorf("Could not read macaroon %v", path)
	}

	hexMacaroon := hex.EncodeToString(macaroonBytes)

	return hexMacaroon, nil
}
