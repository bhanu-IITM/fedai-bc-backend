package fabric

import (
	"fmt"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/hash"
	"google.golang.org/grpc"
)

type Gateway struct {
	cfg     Config
	conn    *grpc.ClientConn
	gateway *client.Gateway
}

func NewGateway(cfg Config) (*Gateway, error) {
	tlsCreds, err := newGrpcTLSCredentials(cfg.TLSCAPath, cfg.TLSHostName)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial(cfg.PeerEndpoint, grpc.WithTransportCredentials(tlsCreds))
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", cfg.PeerEndpoint, err)
	}

	id, sign, err := loadIdentityAndSign(cfg.MSPID, cfg.SignCertPath, cfg.SignKeyPath)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	gw, err := client.Connect(
		id,
		client.WithSign(sign),
		client.WithHash(hash.SHA256),
		client.WithClientConnection(conn),
		client.WithEvaluateTimeout(time.Duration(cfg.EvaluateTimeoutSec)*time.Second),
		client.WithEndorseTimeout(time.Duration(cfg.EndorseTimeoutSec)*time.Second),
		client.WithSubmitTimeout(time.Duration(cfg.SubmitTimeoutSec)*time.Second),
		client.WithCommitStatusTimeout(time.Duration(cfg.CommitTimeoutSec)*time.Second),
	)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("gateway connect: %w", err)
	}

	return &Gateway{cfg: cfg, conn: conn, gateway: gw}, nil
}

func (g *Gateway) Close() {
	if g.gateway != nil {
		_ = g.gateway.Close()
	}
	if g.conn != nil {
		_ = g.conn.Close()
	}

}

func (g *Gateway) Contract() *client.Contract {
	network := g.gateway.GetNetwork(g.cfg.Channel)
	return network.GetContract(g.cfg.Chaincode)
}
