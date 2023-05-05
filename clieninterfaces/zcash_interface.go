/** diablo-benchmark-2\blockchains\clientinterfaces\zcash_interface.go */

/** Presently, this code is mostly copied from ./solana_interface.go, but
this file has comments for where the changes need to be made.
Comments are not fully finished. */

package clientinterfaces

import (
	"context"
	"diablo-benchmark/blockchains/workloadgenerators"
	"diablo-benchmark/core/configs"
	"diablo-benchmark/core/results"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	/** need to update imports */
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"go.uber.org/zap"
)

/** Here are the Ethereum imports, for example:
ethtypes "github.com/ethereum/go-ethereum/core/types"
"github.com/ethereum/go-ethereum/ethclient"
"go.uber.org/zap"
*/

/** TODO: update utils.go */

/** Below are some structs that are used throughout this file.

This is a chainConfig:

// ChainConfig contains the information about the blockchain configuration file
type ChainConfig struct {
	Name             string        `yaml:name` // Name of the chain (will be used in config print)
	Path             string        // Path of the configuration file
	Nodes            []string      `yaml:nodes`                // Address of the nodes.
	KeyFile          string        `yaml:"key_file,omitempty"` // JSON file with privkey:address pairs
	ThroughputWindow int           `yaml:"window"`             // Window for thropughput calculation (default 1s)
	Keys             []ChainKey    `yaml:keys,flow`            // Key information
	Extra            []interface{} `yaml:"extra,flow,omitempty"`
}

And this is a GenericBlock:

// GenericBlock defines a generic block structure for the blockchains, this may or may not be fully filled.
// This should be extended to accompany for other blockchains but MUST retain
// base functionality for other chains.
type GenericBlock struct {
	Hash              string   // Unique identifier for the block
	Index             uint64   // Height of the block as an index
	Timestamp         uint64   // Unix timestamp of the block
	TransactionNumber int      // Number of transactions included in the block
	TransactionHashes []string // The hash of each transaction included in the block
}
*/

type ZcashInterface struct {
	/** Possible implementation here instead of Connections
	PrimaryConnection *rpc.Client /** Ethereum uses *ethclient.Client
	SecondaryConnections []*rpc.Client
	*/
	Connections      []*rpc.Client // Active connections to a blockchain node for information TODO: get this right
	NextConnection   uint64
	SubscribeDone    chan bool              // Event channel that will unsub from events
	TransactionInfo  map[string][]time.Time // Transaction information TODO: make work with zcash. Type may need to change
	bigLock          sync.Mutex
	HandlersStarted  bool         // Have the handlers been initiated?
	StartTime        time.Time    // Start time of the benchmark
	ThroughputTicker *time.Ticker // Ticker for throughput (1s)
	Throughputs      []float64    // Throughput over time with 1 second intervals
	logger           *zap.Logger
	GenericInterface
}

// TODO: check where each of these objects point and make sure this works. See Solana
func (z *ZcashInterface) ActiveConn() *rpc.Client {
	i := atomic.AddUint64(&z.NextConnection, 1)
	client := s.Connections[i%uint64(len(z.Connections))]
	return client
}

func NewZcashInterface() *ZcashInterface {
	return &ZcashInterface{logger: zap.L().Named("ZcashInterface")}
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) Init(chainConfig *configs.ChainConfig) {
	z.logger.Debug("Init")
	z.Nodes = chainConfig.Nodes
	z.TransactionInfo = make(map[string][]time.Time, 0) // TODO: see: TransactionInfo above
	z.SubscribeDone = make(chan bool)
	z.HandlersStarted = false
	z.NumTxDone = 0
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) Cleanup() results.Results {
	z.logger.Debug("Cleanup")
	// Stop the ticker
	z.ThroughputTicker.Stop()

	// clean up connections and format results
	if z.HandlersStarted {
		z.SubscribeDone <- true
	}

	txLatencies := make([]float64, 0)
	var avgLatency float64

	var endTime time.Time

	success := uint(0)
	fails := uint(z.Fail)

	for _, v := range z.TransactionInfo {
		if len(v) > 1 {
			/** Check time until the next time the transaction was handled, which is the latency */
			txLatency := v[1].Sub(v[0]).Milliseconds()
			txLatencies = append(txLatencies, float64(txLatency))
			avgLatency += float64(txLatency)
			if v[1].After(endTime) {
				endTime = v[1]
			}

			success++
		} else {
			/** The transaction was never handled again; it failed! */
			/** See Solana or Ethereum for how to handle this; currently is like Ethereum */
			fails++
		}
	}

	z.logger.Debug("Statistics being returned",
		zap.Uint("success", success),
		zap.Uint("fail", fails))

	// Calculate the throughput and latencies
	var throughput float64
	if len(txLatencies) > 0 {
		throughput = (float64(z.NumTxDone) - float64(z.Fail)) / (endTime.Sub(z.StartTime).Seconds())
		avgLatency = avgLatency / float64(len(txLatencies))
	} else {
		avgLatency = 0
		throughput = 0
	}

	averageThroughput := float64(0)
	var calculatedThroughputSeconds = []float64{z.Throughputs[0]}
	for i := 1; i < len(z.Throughputs); i++ {
		calculatedThroughputSeconds = append(calculatedThroughputSeconds, float64(z.Throughputs[i]-z.Throughputs[i-1]))
		averageThroughput += float64(z.Throughputs[i] - z.Throughputs[i-1])
	}

	averageThroughput = averageThroughput / float64(len(z.Throughputs))

	z.logger.Debug("Results being returned",
		zap.Float64("avg throughput", averageThroughput),
		zap.Float64("throughput (as is)", throughput),
		zap.Float64("latency", avgLatency),
		zap.String("ThroughputWindow", fmt.Sprintf("%v", calculatedThroughputSeconds)),
	)

	return results.Results{
		TxLatencies:       txLatencies,
		AverageLatency:    avgLatency,
		Throughput:        averageThroughput,
		ThroughputSeconds: calculatedThroughputSeconds,
		Success:           success,
		Fail:              fails,
	}
}

/** This is currently unchanged from Solana and Ethereum */
func (z *ZcashInterface) throughputSeconds() {
	s.ThroughputTicker = time.NewTicker(time.Duration(z.Window) * time.Second)
	seconds := float64(0)

	for range z.ThroughputTicker.C {
		seconds += float64(z.Window)
		z.Throughputs = append(z.Throughputs, float64(z.NumTxDone-z.Fail))
	}
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) Start() {
	z.logger.Debug("Start")
	z.StartTime = time.Now()
	go z.throughputSeconds()
}

/** TODO: update transactions to work with Zcash. Only the two commented lines need to change. */
/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) ParseWorkload(workload workloadgenerators.WorkerThreadWorkload) ([][]interface{}, error) {
	z.logger.Debug("ParseWorkload")
	parsedWorkload := make([][]interface{}, 0)

	for _, v := range workload {
		intervalTxs := make([]interface{}, 0)
		for _, txBytes := range v {
			/** Change the following two lines */
			t := solana.Transaction{}          /** Need to get zcash transaction here */            /** t := ethtypes.Transaction{} */
			err := json.Unmarshal(txBytes, &t) /** Need to make this work with Zcash transaction */ /** err := t.UnmarshalJSON(txBytes) */
			/** May want to check core/configs/types.go for UnmarshalJSON */
			if err != nil {
				return nil, err
			}

			intervalTxs = append(intervalTxs, &t)
		}
		parsedWorkload = append(parsedWorkload, intervalTxs)
	}

	z.TotalTx = len(parsedWorkload)

	return parsedWorkload, nil
}

/** I think Zcash has a GetBlockByNumber function, so we could use that like Ethereum */
// parseBlocksForTransactions parses the the given block number for the transactions
func (z *ZcashInterface) parseBlocksForTransactions(slot uint64) {
	z.logger.Debug("parseBlocksForTransactions", zap.Uint64("slot", slot))

	/** TODO: update this based on way connections are stored */
	/** interface variable is left as `s` because it is unchanged from Solana */
	var block *rpc.GetBlockResult /* TODO */
	var err error
	for attempt := 0; attempt < 100; attempt++ {
		includeRewards := false
		block, err = s.ActiveConn().rpcClient.GetBlockWithOpts(
			context.Background(),
			slot,
			&rpc.GetBlockOpts{
				TransactionDetails: rpc.TransactionDetailsSignatures,
				Rewards:            &includeRewards,
				Commitment:         rpc.CommitmentFinalized,
			})

		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if block == nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		break
	}

	/** For the above code, Ethereum just calles the primary connection's blockbynumber function, as in the following line */
	/** block, err := e.PrimaryNode.BlockByNumber(context.Background(), blockNumber) */

	tNow := time.Now()
	var tAdd uint64

	z.bigLock.Lock()

	/** TODO based on transaction info */
	for _, sig := range block.Signatures {
		if info, ok := s.TransactionInfo[sig]; ok && len(info) == 1 {
			s.TransactionInfo[sig] = append(info, tNow)
			tAdd++
		}
	}

	/** Here is the Ethereum way to do the above */
	/**
	for _, v := range block.Transactions() {
		tHash := v.Hash().String()
		if _, ok := e.TransactionInfo[tHash]; ok {
			e.TransactionInfo[tHash] = append(e.TransactionInfo[tHash], tNow)
			tAdd++
		}
	}
	*/

	z.bigLock.Unlock()

	atomic.AddUint64(&z.NumTxDone, tAdd)
	z.logger.Debug("Stats", zap.Uint64("sent", atomic.LoadUint64(&z.NumTxSent)), zap.Uint64("done", atomic.LoadUint64(&z.NumTxDone)))
}

/** This implementation will depend heavily on the returned subscription type */
/** It seems like this will call parseBlocksForTransactions on the incoming information from the subscription */
// EventHandler subscribes to the blocks and handles the incoming information about the transactions
func (z *ZcashInterface) EventHandler() {
	z.logger.Debug("EventHandler")
	sub, err := z.ActiveConn().wsClient.RootSubscribe() /** TODO: Update for Zcash client connection */
	if err != nil {
		s.logger.Warn("RootSubscribe", zap.Error(err))
		return
	}
	defer sub.Unsubscribe()
	go func() {
		for range s.SubscribeDone {
			sub.Unsubscribe()
			return
		}
	}()

	var currentSlot uint64 = 0
	for {
		got, err := sub.Recv()
		if err != nil {
			s.logger.Warn("RootResult", zap.Error(err))
			return
		}
		if got == nil {
			s.logger.Warn("Empty root")
			return
		}
		if currentSlot == 0 {
			s.logger.Debug("First slot", zap.Uint64("got", uint64(*got)))
		} else if uint64(*got) <= currentSlot {
			s.logger.Debug("Slot skipped", zap.Uint64("got", uint64(*got)), zap.Uint64("current", currentSlot))
			continue
		} else if uint64(*got) > currentSlot+1 {
			s.logger.Fatal("Missing slot update", zap.Uint64("got", uint64(*got)), zap.Uint64("current", currentSlot))
		}
		currentSlot = uint64(*got)
		// Got a head
		go s.parseBlocksForTransactions(uint64(*got))
	}
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (s *SolanaInterface) ConnectOne(id int) error {
	s.logger.Debug("ConnectOne")
	return errors.New("do not use")
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (s *SolanaInterface) ConnectAll(primaryID int) error {
	s.logger.Debug("ConnectAll")
	// If our ID is greater than the nodes we know, there's a problem!
	if primaryID >= len(s.Nodes) {
		return errors.New("invalid client primary ID")
	}

	// Connect all the others
	for _, node := range s.Nodes {
		conn := rpc.New(fmt.Sprintf("http://%s", node))

		ip, portStr, err := net.SplitHostPort(node)
		if err != nil {
			return err
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return err
		}

		sock, err := ws.Connect(context.Background(), fmt.Sprintf("ws://%s", net.JoinHostPort(ip, strconv.Itoa(port+1))))
		if err != nil {
			return err
		}

		s.Connections = append(s.Connections, &solanaClient{conn, sock})
	}

	if !s.HandlersStarted {
		go s.EventHandler()
		s.HandlersStarted = true
	}

	return nil
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (s *SolanaInterface) DeploySmartContract(tx interface{}) (interface{}, error) {
	s.logger.Debug("DeploySmartContract")
	return nil, errors.New("not implemented")
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (s *SolanaInterface) SendRawTransaction(tx interface{}) error {
	go func() {
		transaction := tx.(*solana.Transaction)

		sig, err := s.ActiveConn().rpcClient.SendTransactionWithOpts(context.Background(), transaction, false, rpc.CommitmentFinalized)
		if err != nil {
			s.logger.Debug("Err",
				zap.Error(err),
			)
			atomic.AddUint64(&s.Fail, 1)
			atomic.AddUint64(&s.NumTxDone, 1)
		}

		s.bigLock.Lock()
		s.TransactionInfo[sig] = []time.Time{time.Now()}
		s.bigLock.Unlock()

		atomic.AddUint64(&s.NumTxSent, 1)
	}()

	return nil
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (s *SolanaInterface) SecureRead(callFunc string, callParams []byte) (interface{}, error) {
	s.logger.Debug("SecureRead")
	return nil, errors.New("not implemented")
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (s *SolanaInterface) GetBlockByNumber(index uint64) (GenericBlock, error) {
	s.logger.Debug("GetBlockByNumber")
	return GenericBlock{}, errors.New("not implemented")
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (s *SolanaInterface) GetBlockHeight() (uint64, error) {
	s.logger.Debug("GetBlockHeight")
	return 0, errors.New("not implemented")
}

/** If we can get block by number, then we just go through the block numbers and parse */
/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (s *SolanaInterface) ParseBlocksForTransactions(startNumber uint64, endNumber uint64) error {
	s.logger.Debug("ParseBlocksForTransactions")
	return errors.New("not implemented")
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (s *SolanaInterface) Close() {
	s.logger.Debug("Close")
	// Close all connections
	for _, client := range s.Connections {
		client.wsClient.Close()
	}
}

/**

REQUIRED FOR BLOCKCHAIN_INTERFACE BUT NOT IMPLEMENTED HERE:
GetTxDone() uint64 // already implemented with GenericInterface
SetWindow(window int) // no comment on if this is already implemented

*/
