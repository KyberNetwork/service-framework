package client

import (
	"context"
	"math/big"
	"net/http"
	"time"

	"github.com/KyberNetwork/kutils/klog"
	"github.com/KyberNetwork/kyber-trace-go/pkg/tracer"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const EthCloseDelay = time.Minute

type EthCfg struct {
	Url        string
	ArchiveUrl string
	C          *EthClient
}

func (*EthCfg) OnUpdate(old, new *EthCfg) {
	ctx := context.Background()
	if old != nil && old.C != nil {
		time.AfterFunc(EthCloseDelay, func() {
			old.C.Close()
		})
	}
	var err error
	new.C, err = new.Dial(context.Background())
	if err != nil {
		klog.Errorf(ctx, "EthCfg.OnUpdate|new.Dial failed: %v", err)
	}
}

func (c *EthCfg) Dial(ctx context.Context) (*EthClient, error) {
	ethCli, err := Dial(ctx, c.Url)
	if err != nil {
		err = errors.Wrapf(err, "EthCfg.Dial %s", c.Url)
	}
	archiveEthCli, archiveErr := Dial(ctx, c.ArchiveUrl)
	if archiveErr != nil {
		archiveErr = errors.Wrapf(archiveErr, "EthCfg.Dial %s", c.ArchiveUrl)
	}
	if err != nil || ethCli == nil {
		if archiveErr != nil || archiveEthCli == nil {
			return nil, err
		}
		ethCli = archiveEthCli
	} else if archiveErr != nil || archiveEthCli == nil {
		archiveEthCli = ethCli
	}

	return &EthClient{Client: ethCli, Archive: archiveEthCli}, nil
}

func Dial(ctx context.Context, url string) (*ethclient.Client, error) {
	if tracer.Provider() == nil {
		return ethclient.DialContext(ctx, url)
	}
	rpcClient, err := rpc.DialOptions(ctx, url,
		rpc.WithHTTPClient(&http.Client{Transport: otelhttp.NewTransport(nil)}))
	if err != nil {
		return nil, err
	}
	return ethclient.NewClient(rpcClient), nil
}

type EthClient struct {
	*ethclient.Client
	Archive *ethclient.Client
}

func (c *EthClient) Close() {
	c.Client.Close()
	c.Archive.Close()
}

func (c *EthClient) ClientFor(blockNumber *big.Int) *ethclient.Client {
	if blockNumber == nil {
		return c.Client
	}
	return c.Archive
}

func (c *EthClient) BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error) {
	return c.ClientFor(blockNumber).BalanceAt(ctx, account, blockNumber)
}

func (c *EthClient) CodeAt(ctx context.Context, account common.Address, blockNumber *big.Int) ([]byte, error) {
	return c.ClientFor(blockNumber).CodeAt(ctx, account, blockNumber)
}

func (c *EthClient) NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error) {
	return c.ClientFor(blockNumber).NonceAt(ctx, account, blockNumber)
}

func (c *EthClient) StorageAt(ctx context.Context, account common.Address, key common.Hash,
	blockNumber *big.Int) ([]byte, error) {
	return c.ClientFor(blockNumber).StorageAt(ctx, account, key, blockNumber)
}

func (c *EthClient) CallContract(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error) {
	return c.ClientFor(blockNumber).CallContract(ctx, call, blockNumber)
}
