/** diablo-benchmark-2\blockchains\clientinterfaces\zcash_interface.go */

/** Presently, this code is mostly copied from ./solana_interface.go, but
this file has comments for where the changes need to be made.
Comments are not fully finished. */

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

/**
btcjson/chainsvrresults.go

// TxRawResult models the data from the getrawtransaction command.
type TxRawResult struct {
	Hex           string `json:"hex"`
	Txid          string `json:"txid"`
	Hash          string `json:"hash,omitempty"`
	Size          int32  `json:"size,omitempty"`
	Vsize         int32  `json:"vsize,omitempty"`
	Weight        int32  `json:"weight,omitempty"`
	Version       uint32 `json:"version"`
	LockTime      uint32 `json:"locktime"`
	Vin           []Vin  `json:"vin"`
	Vout          []Vout `json:"vout"`
	BlockHash     string `json:"blockhash,omitempty"`
	Confirmations uint64 `json:"confirmations,omitempty"`
	Time          int64  `json:"time,omitempty"`
	Blocktime     int64  `json:"blocktime,omitempty"`
}

// GetBlockVerboseResult models the data from the getblock command when the
// verbose flag is set to 1.  When the verbose flag is set to 0, getblock returns a
// hex-encoded string. When the verbose flag is set to 1, getblock returns an object
// whose tx field is an array of transaction hashes. When the verbose flag is set to 2,
// getblock returns an object whose tx field is an array of raw transactions.
// Use GetBlockVerboseTxResult to unmarshal data received from passing verbose=2 to getblock.
type GetBlockVerboseResult struct {
	Hash          string        `json:"hash"`
	Confirmations int64         `json:"confirmations"`
	StrippedSize  int32         `json:"strippedsize"`
	Size          int32         `json:"size"`
	Weight        int32         `json:"weight"`
	Height        int64         `json:"height"`
	Version       int32         `json:"version"`
	VersionHex    string        `json:"versionHex"`
	MerkleRoot    string        `json:"merkleroot"`
	Tx            []string      `json:"tx,omitempty"`
	RawTx         []TxRawResult `json:"rawtx,omitempty"` // Note: this field is always empty when verbose != 2.
	Time          int64         `json:"time"`
	Nonce         uint32        `json:"nonce"`
	Bits          string        `json:"bits"`
	Difficulty    float64       `json:"difficulty"`
	PreviousHash  string        `json:"previousblockhash"`
	NextHash      string        `json:"nextblockhash,omitempty"`
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
	z.TransactionInfo = make(txinfo, 0) /** txinfo is alias right now, so maybe this won't work? */
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

	atomic.AddUint64(&z.NumTxDone, tAdd) /** why not add before the unlock */
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
			/** TODO: update notification settings from client */
			return
		case hash := <- z.HashChannel:
			go z.parseBlockForTransactions(hash)
		}
	}
}

var placeholder string = ""

func (z *ZcashInterface) OnBlockConnected (hash *chainhash.Hash, height int32, t time.Time) {
	/** TODO: do we need to lock? */
	z.HashChannel <- hash
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) ConnectOne(id int) error {
	/** id is the index in the nodes list. It's not actually an 'identification' */

	// If our ID is greater than the nodes we know, there's a problem!
	if id >= len(z.Nodes) {
		return errors.New("invalid client ID")
	}

	/** TODO: this is probably broken. See more about ConnConfig */
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

	/** TODO: Is hash.String() the same as TxRawResult.Hash ? See for loop in z.ParseBlockForTransaction */
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
	for _, client := range z.SecondaryConnections {
		client.Disconnect()
	}
}

/**

REQUIRED FOR BLOCKCHAIN_INTERFACE BUT NOT IMPLEMENTED HERE:
GetTxDone() uint64 // already implemented with GenericInterface
SetWindow(window int) // already implemented with GenericInterface

*/
