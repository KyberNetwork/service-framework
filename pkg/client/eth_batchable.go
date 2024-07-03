package client

import (
	"context"
	"math/big"
	"time"

	"github.com/KyberNetwork/kutils"
	"github.com/cenkalti/backoff/v4"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

// BatchableEthCfg is hotcfg for batchable eth client.
// It batches eth_call's up to be sent together within 1 request to the rpc node.
type BatchableEthCfg struct {
	EthCfg    `mapstructure:",squash"`
	BatchRate time.Duration
	BatchCnt  int
	BackOff   *BackoffCfg
	C         *BatchableEthClient
}

func (*BatchableEthCfg) OnUpdate(old, new *BatchableEthCfg) {
	if old == nil {
		new.EthCfg.OnUpdate(nil, &new.EthCfg)
		new.BackOff.OnUpdate(nil, new.BackOff)
	} else {
		new.EthCfg.OnUpdate(&old.EthCfg, &new.EthCfg)
		new.BackOff.OnUpdate(old.BackOff, new.BackOff)
	}
	if old != nil && old.C != nil {
		oldC := old.C
		time.AfterFunc(EthCloseDelay, func() {
			oldC.Close()
		})
	}
	new.C = NewBatchableEthClient(new.EthCfg.C, func() (time.Duration, int) {
		return new.BatchRate, new.BatchCnt
	}, new.BackOff.BackOff)
}

type BatchableEthClient struct {
	*EthClient
	batcher        kutils.Batcher[*CallMsg, []byte]
	archiveBatcher kutils.Batcher[*CallMsg, []byte]
	backOff        backoff.BackOff
}

type CallMsg struct {
	ethereum.CallMsg
	BlockNumber *big.Int
	*kutils.ChanTask[[]byte]
}

func NewBatchableEthClient(client *EthClient, batchCfg kutils.BatchCfg, backOff backoff.BackOff) *BatchableEthClient {
	batchable := &BatchableEthClient{
		EthClient: client,
		backOff:   backOff,
	}
	batchable.batcher = kutils.NewChanBatcher[*CallMsg, []byte](batchCfg, batchable.batchCalls)
	batchable.archiveBatcher = kutils.NewChanBatcher[*CallMsg, []byte](batchCfg, batchable.batchCalls)
	return batchable
}

func (b *BatchableEthClient) CallContract(ctx context.Context, msg ethereum.CallMsg, blockNumber *big.Int) ([]byte,
	error) {
	task := &CallMsg{
		CallMsg:     msg,
		BlockNumber: blockNumber,
		ChanTask:    kutils.NewChanTask[[]byte](ctx),
	}
	if blockNumber != nil {
		b.archiveBatcher.Batch(task)
	} else {
		b.batcher.Batch(task)
	}
	return task.Result()
}

func (b *BatchableEthClient) Close() {
	b.EthClient.Close()
	b.batcher.Close()
	b.archiveBatcher.Close()
}

func (b *BatchableEthClient) batchCalls(msgs []*CallMsg) {
	if len(msgs) == 0 {
		return
	}
	ctx := kutils.CtxWithoutCancel(msgs[len(msgs)-1].Ctx())
	reqs := make([]rpc.BatchElem, len(msgs))
	for i, msg := range msgs {
		reqs[i] = rpc.BatchElem{
			Method: "eth_call",
			Args:   []any{toCallArg(msg.CallMsg), toBlockNumArg(msg.BlockNumber)},
			Result: new(hexutil.Bytes),
		}
	}
	if err := backoff.Retry(func() error {
		return b.EthClient.ClientFor(msgs[0].BlockNumber).Client().BatchCallContext(ctx, reqs)
	}, b.backOff); err != nil {
		for _, msg := range msgs {
			msg.Resolve(nil, err)
		}
		return
	}
	for i, req := range reqs {
		msgs[i].Resolve(*req.Result.(*hexutil.Bytes), req.Error)
	}
}

func toCallArg(msg ethereum.CallMsg) any {
	arg := map[string]any{
		"from": msg.From,
		"to":   msg.To,
	}
	if len(msg.Data) > 0 {
		arg["data"] = hexutil.Bytes(msg.Data)
	}
	if msg.Value != nil {
		arg["value"] = (*hexutil.Big)(msg.Value)
	}
	if msg.Gas != 0 {
		arg["gas"] = hexutil.Uint64(msg.Gas)
	}
	if msg.GasPrice != nil {
		arg["gasPrice"] = (*hexutil.Big)(msg.GasPrice)
	}
	return arg
}

func toBlockNumArg(number *big.Int) string {
	if number == nil {
		return "latest"
	}
	pending := big.NewInt(-1)
	if number.Cmp(pending) == 0 {
		return "pending"
	}
	finalized := big.NewInt(int64(rpc.FinalizedBlockNumber))
	if number.Cmp(finalized) == 0 {
		return "finalized"
	}
	safe := big.NewInt(int64(rpc.SafeBlockNumber))
	if number.Cmp(safe) == 0 {
		return "safe"
	}
	return hexutil.EncodeBig(number)
}
