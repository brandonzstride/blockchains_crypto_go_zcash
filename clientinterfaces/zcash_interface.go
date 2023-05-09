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

	zap "go.uber.org/zap"

	rpc "github.com/arithmetric/zcashrpcclient"
	zrpc "github.com/arithmetric/zcashrpcclient"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc/ws"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/websocket" /** for websocket.Conn */
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

/** TODO: this needs to be integrated with zrpc somehow */
type zcashClient struct {
	wsconn websocket.Conn
}

type txinfo map[string][]time.Time /** string is key because btcjson.TxRawResult has Hash field of type string */

type ZcashInterface struct {
	PrimaryConnection    *zcashClient
	SecondaryConnections []*zcashClient
	IsBlockSeen          map[string]bool // key is block hash
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
	Nodes                []string // TODO: replace string with actual type of node
	Connections          []*zcashClient
	// ActiveConn           websocket.Conn
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
			var t btcjson.TxRawResult /** this custom for Zcash */
			/** NOTE: z.PrimaryConnection is a struct and not actually a Zcash client! We need to put a client field into it */
			t, err := z.PrimaryConnection.DecodeRawTransaction(txbytes) /** this custom for Zcash */
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

// parseBlockForTransactions parses the given block for the transactions
func (z *ZcashInterface) parseBlockForTransactions(block btcjson.GetBlockVerboseTxResult) {
	if z.IsBlockSeen[block.Hash] {
		return
	}

	/** Does this get set to true whenever ANY node in the Diablo network sees the block? Or is this map unique for each specific node */
	/** It's probably the latter because this code seems like it gets run from a secondary, which is one(?) node */
	z.IsBlockSeen[block.Hash] = true

	tNow := time.Now()
	var tAdd uint64

	z.bigLock.Lock()

	for _, v := range block.Tx {
		tHash := v.Hash()
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
	z.logger.Debug("parseBestBlockForTransactions", zap.Uint64("slot", slot))

	/** NOTE: z.PrimaryConnection is a struct and not actually a Zcash client! We need to put a client field into it */
	hash, err := z.PrimaryConnection.GetBestBlockHash()

	if err != nil {
		z.logger.Warn(err.Error())
		return
	}

	/** NOTE: z.PrimaryConnection is a struct and not actually a Zcash client! We need to put a client field into it */
	block, err := z.PrimaryConnection.GetBlockVerboseTx(hash) /** models getblock when verbose = 2, so this contains all transations */

	if err != nil {
		z.logger.Warn(err.Error())
		return
	}

	z.parseBlockForTransactions(block)
}

// EventHandler subscribes to the blocks and handles the incoming information about the transactions
func (z *ZcashInterface) EventHandler() {
	z.logger.Debug("EventHandler")

	/** This may be slow because we have to check if the block has been seen each time, */
	/** but we have no `subscribe` function, so this is the best we can do */
	/** NOTE: z.PrimaryConnection is a struct and not actually a Zcash client! We need to put a client field into it */
	futureBlock := z.PrimaryConnection.getBestBlockVerboseTxAsync()

	for { /** while true, read from channels */
		select {
		case <-z.SubscribeDone: /** Cleanup called <=> time to unsubscribe */
			/** unsubscribe here i.e. stop getting notifications from node via a channel */
			return
		case response <- futureBlock:
			block, err := response.Receive()
			if err != nil {
				z.logger.Warn(err.Error())
				return
			}
			z.parseBlockForTransactions(block)
			/** NOTE: z.PrimaryConnection is a struct and not actually a Zcash client! We need to put a client field into it */
			futureBlock = z.PrimaryConnection.getBestBlockVerboseTxAsync() /* ask for another block */
		case err := sub.Err():
			z.logger.Warn(err.Error())
			return /** maybe don't return on an error? */
		}
	}
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) ConnectOne(id int) error {
	/** id is the index in the nodes list. It's not actually an 'identification' */

	// If our ID is greater than the nodes we know, there's a problem!
	if id >= len(z.Nodes) {
		return errors.New("invalid client ID")
	}

	// Connect to the node
	conn, err := zrpc.Dial(connectionConfig) /** TODO: fill this! */

	// c, err := ethclient.Dial(fmt.Sprintf("ws://%s", e.Nodes[id]))

	// // If there's an error, raise it.
	// if err != nil {
	// 	return err
	// }

	// e.PrimaryNode = c

	// if !e.HandlersStarted {
	// 	go e.EventHandler()
	// 	e.HandlersStarted = true
	// }

	return nil
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) ConnectAll(primaryID int) error {
	z.logger.Debug("ConnectAll")
	// If our ID is greater than the nodes we know, there's a problem!
	if primaryID >= len(z.Nodes) {
		return errors.New("invalid client primary ID")
	}

	// Connect all the others
	for _, node := range z.Nodes {
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

		z.Connections = append(z.Connections, &zcashClient{conn, sock})
	}

	if !z.HandlersStarted {
		go z.EventHandler()
		z.HandlersStarted = true
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
	go func() {
		transaction := tx.(*solana.Transaction)

		sig, err := z.ActiveConn().rpcClient.SendTransactionWithOpts(context.Background(), transaction, false, rpc.CommitmentFinalized)
		if err != nil {
			z.logger.Debug("Err",
				zap.Error(err),
			)
			atomic.AddUint64(&z.Fail, 1)
			atomic.AddUint64(&z.NumTxDone, 1)
		}

		z.bigLock.Lock()
		z.TransactionInfo[sig] = []time.Time{time.Now()}
		z.bigLock.Unlock()

		atomic.AddUint64(&z.NumTxSent, 1)
	}()

	return nil
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) SecureRead(callFunc string, callParams []byte) (interface{}, error) {
	z.logger.Debug("SecureRead")
	return nil, errors.New("not implemented")
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) GetBlockByNumber(index uint64) (GenericBlock, error) {
	z.logger.Debug("GetBlockByNumber")
	return GenericBlock{}, errors.New("not implemented")
}

/** REQUIRED FOR BLOCKCHAIN_INTERFACE */
func (z *ZcashInterface) GetBlockHeight() (uint64, error) {
	z.logger.Debug("GetBlockHeight")
	return 0, errors.New("not implemented")
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
	for _, client := range z.Connections {
		client.wsconn.Close()
	}
}

/**

REQUIRED FOR BLOCKCHAIN_INTERFACE BUT NOT IMPLEMENTED HERE:
GetTxDone() uint64 // already implemented with GenericInterface
SetWindow(window int) // no comment on if this is already implemented

*/
