/** diablo-benchmark-2\blockchains\clientinterfaces\zcash_interface.go */

package clientinterfaces

import (
	"diablo-benchmark/blockchains/workloadgenerators"
	"diablo-benchmark/core/configs"
	"diablo-benchmark/core/results"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	zap "go.uber.org/zap"

	// rpc "github.com/arithmetric/zcashrpcclient"
	rpc "diablo-benchmark/zcashrpcclient"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

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
*/

/**
btcjson/chainsvrresults.go

type TxRawResult struct {
	..
	Hash          string `json:"hash,omitempty"`
	..
}

type GetBlockVerboseResult struct {
	..
	RawTx         []TxRawResult `json:"rawtx,omitempty"`
	..
}

*/

type zcashClient = rpc.Client

type txinfo map[string][]time.Time /** string is key because btcjson.TxRawResult has Hash field of type string */

type ZcashInterface struct {
	PrimaryConnection    *zcashClient
	SecondaryConnections []*zcashClient
	SubscribeDone        chan bool       // Event channel that will unsub from events
	TransactionInfo      txinfo          // Transaction information // keep key as a string to stay universal
	bigLock              sync.Mutex
	HandlersStarted      bool         // Have the handlers been initiated?
	StartTime            time.Time    // Start time of the benchmark
	ThroughputTicker     *time.Ticker // Ticker for throughput (1s)
	Throughputs          []float64    // Throughput over time with 1 second intervals
	logger               *zap.Logger
	Fail                 uint64
	NumTxDone            uint64
	HashChannel		 	 chan *chainhash.Hash // channel for new blocks
	GenericInterface
}

func NewZcashInterface() *ZcashInterface {
	return &ZcashInterface{logger: zap.L().Named("ZcashInterface")}
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) Init(chainConfig *configs.ChainConfig) {
	z.logger.Debug("Init Zcash interface")
	z.Nodes = chainConfig.Nodes
	z.TransactionInfo = make(txinfo, 0)
	z.SubscribeDone = make(chan bool)
	z.HandlersStarted = false
	z.NumTxDone = 0
	z.HashChannel = make(chan *chainhash.Hash)
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

/** Ticker starts, and this fills z.Throughputs with the number of transactions that succeeded between each tick */
func (z *ZcashInterface) throughputSeconds() {
	z.ThroughputTicker = time.NewTicker(time.Duration(z.Window) * time.Second)
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
	go z.throughputSeconds() /** start goroutine on ticker */
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) ParseWorkload(workload workloadgenerators.WorkerThreadWorkload) ([][]interface{}, error) {
	z.logger.Debug("ParseWorkload")
	parsedWorkload := make([][]interface{}, 0)

	for _, v := range workload {
		intervalTxs := make([]interface{}, 0)
		for _, txBytes := range v {
			var t *btcjson.TxRawResult /** this custom for Zcash */
			t, err := z.PrimaryConnection.DecodeRawTransaction(txBytes) /** this custom for Zcash */

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

// parseBlockForTransactions parses the given block hash for the transactions
func (z *ZcashInterface) parseBlockForTransactions(hash *chainhash.Hash) {
	block, err := z.PrimaryConnection.GetBlockVerboseTx(hash) /** models getblock when verbose = 2, so this contains all transations */

	if err != nil {
		z.logger.Warn(err.Error())
		return
	}

	tNow := time.Now()
	var tAdd uint64

	z.bigLock.Lock()

	for _, v := range block.RawTx {
		tHash := v.Hash
		if _, ok := z.TransactionInfo[tHash]; ok {
			z.TransactionInfo[tHash] = append(z.TransactionInfo[tHash], tNow)
			tAdd++
		}
	}

	z.bigLock.Unlock()

	atomic.AddUint64(&z.NumTxDone, tAdd)
	z.logger.Debug("Stats", zap.Uint64("sent", atomic.LoadUint64(&z.NumTxSent)), zap.Uint64("done", atomic.LoadUint64(&z.NumTxDone)))
}

// parseBlocksForTransactions parses the most recent block for transactions
func (z *ZcashInterface) parseBestBlockForTransactions() {
	hash, err := z.PrimaryConnection.GetBestBlockHash()

	if err != nil {
		z.logger.Warn(err.Error())
		return
	}

	z.parseBlockForTransactions(hash)
}

// EventHandler subscribes to the blocks and handles the incoming information about the transactions
func (z *ZcashInterface) EventHandler() {
	z.logger.Debug("EventHandler")

	/** We've already subscribed when we connected via the HashChannel channel */

	for { /** while true, read from channels */
		select {
		case <- z.SubscribeDone: /** Cleanup called <=> time to unsubscribe */
			/** unsubscribe here i.e. stop getting notifications from node via a channel */
			return
		case hash := <- z.HashChannel:
			go z.parseBlockForTransactions(hash)
		}
	}
}

var placeholder string = ""

func (z *ZcashInterface) OnBlockConnected (hash *chainhash.Hash, height int32, t time.Time) {
	z.HashChannel <- hash
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) ConnectOne(id int) error {
	/** id is the index in the nodes list. It's not actually an 'identification' */

	// If our ID is greater than the nodes we know, there's a problem!
	if id >= len(z.Nodes) {
		return errors.New("invalid client ID")
	}

	/** See more about ConnConfig */
	/** https://github.com/arithmetric/zcashrpcclient/blob/7fe0a7b794884635a30971f682db368f8ba3bd8e/infrastructure.go#L1051 */
	connectionConfig := rpc.ConnConfig{Host:z.Nodes[id], Endpoint:"ws", User:placeholder, Pass:placeholder,
									DisableTLS:true, Proxy:""}

	client, err := rpc.New(&connectionConfig, &rpc.NotificationHandlers{OnBlockConnected:z.OnBlockConnected})

	if err != nil {
		return err
	}

	z.PrimaryConnection = client

	if !z.HandlersStarted {
		go z.EventHandler()
		z.HandlersStarted = true
	}

	return nil
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) ConnectAll(primaryID int) error {
	z.logger.Debug("ConnectAll")
	// If our ID is greater than the nodes we know, there's a problem!
	if primaryID >= len(z.Nodes) {
		return errors.New("invalid client primary ID")
	}

	// primary connect
	err := z.ConnectOne(primaryID)

	if err != nil {
		return err
	}

	// Connect all the others
	for idx, node := range z.Nodes {
		if idx != primaryID {
			connectionConfig := rpc.ConnConfig{Host:node, Endpoint:"ws", User:placeholder, Pass:placeholder,
									DisableTLS:true, Proxy:""}
			client, err := rpc.New(&connectionConfig, &rpc.NotificationHandlers{})
			if err != nil {
				return err
			}

			z.SecondaryConnections = append(z.SecondaryConnections, client)
		}
	}

	return nil
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) DeploySmartContract(tx interface{}) (interface{}, error) {
	z.logger.Debug("DeploySmartContract")
	return nil, errors.New("not implemented")
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) SendRawTransaction(tx interface{}) error {
	hash, err := z.PrimaryConnection.SendRawTransaction(tx.(*wire.MsgTx), true)

	if err != nil {
		z.logger.Warn(err.Error())
		atomic.AddUint64(&z.Fail, 1)
		atomic.AddUint64(&z.NumTxDone, 1)
	}

	z.bigLock.Lock()
	z.TransactionInfo[hash.String()] = []time.Time{time.Now()}
	z.bigLock.Unlock()

	atomic.AddUint64(&z.NumTxSent, 1)

	return nil
}

/** apparently not required because Solana and Ethereum don't implement it */
/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) SecureRead(callFunc string, callParams []byte) (interface{}, error) {
	z.logger.Debug("SecureRead")
	return nil, errors.New("not implemented")
}

/** apparently not required because Solana doesn't implement it */
/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) GetBlockByNumber(index uint64) (GenericBlock, error) {
	/** zcashrpc has no way to get a block by number; only by hash or by most recent */
	z.logger.Debug("GetBlockByNumber")
	return GenericBlock{}, errors.New("not implemented")
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) GetBlockHeight() (uint64, error) {
	height, err := z.PrimaryConnection.GetBlockCount()

	if err != nil {
		z.logger.Warn(err.Error())
		return 0, err
	}

	if height < 0 {
		z.logger.Warn(fmt.Sprintf("Got negative block height: %d", height))
	}

	return uint64(height), nil
}

/** If we can get block by number, then we just go through the block numbers and parse */
/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) ParseBlocksForTransactions(startNumber uint64, endNumber uint64) error {
	z.logger.Debug("ParseBlocksForTransactions")
	return errors.New("not implemented")
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) Close() {
	z.logger.Debug("Close")
	// Close all connections
	z.PrimaryConnection.Disconnect()
	for _, client := range z.SecondaryConnections {
		client.Disconnect()
	}
}

/**

REQUIRED FOR BLOCKCHAIN_INTERFACE BUT NOT IMPLEMENTED HERE:
GetTxDone() uint64 // already implemented with GenericInterface
SetWindow(window int) // already implemented with GenericInterface

*/
