## Required RPC calls

Assume that Solana imports begin with "github.com/gagliardetto/solana-go/", and `rpc` is "rpc/", `ws` is "rpc/ws/", and `solana` is the "solana-go/" directory itself. Also assume that `ethclient` is from import "github.com/ethereum/go-ethereum/ethclient" and `ethtypes` is the import "github.com/ethereum/go-ethereum/core/types".

Click on the hyperlinks to see the GitHub code for each RPC call.

* **node connection**. In Solana, this is type `rpc.Client` or `ws.Client`. In Ethereum, this is type `ethclient.Client`.
  * [Ethereum](https://github.com/ethereum/go-ethereum/blob/604e215d1bb070dff98fb76aa965064c74e3633f/ethclient/ethclient.go#L40): connection is achieved via 
  	```go
	c, err := ethclient.Dial(fmt.Sprintf("ws://%s", e.Nodes[id]))
	```
	where `c` is of type `*ethclient.Client`, and `e.Nodes[id]` is string to identify a node.
  * [Solana](https://github.com/gagliardetto/solana-go/blob/290a21adc5d262d93baba0378ebf1dc9a5a1d21d/rpc/client.go#L48): connection is achieved via
  	```go
	conn := rpc.New(fmt.Sprintf("http://%s", node))
	```
	where `node` is a string. It then uses a socket and `ws.Connect` function.
* **Transaction{}**. REFORMAT THIS In [Solana](https://github.com/gagliardetto/solana-go/blob/290a21adc5d262d93baba0378ebf1dc9a5a1d21d/transaction.go#L34), this is
	```go
	t := solana.Transaction{}
	```
	and in [Ethereum](https://github.com/ethereum/go-ethereum/blob/604e215d1bb070dff98fb76aa965064c74e3633f/core/types/transaction.go#L52), this is 
	```go
	t := ethtypes.Transaction{}
	```
  * In Ethereum, `t` has the attribute `UnmarshalJSON` (it is called as `t.UnmarshalJSON(txBytes)`), and code is [here](https://github.com/ethereum/go-ethereum/blob/604e215d1bb070dff98fb76aa965064c74e3633f/core/types/transaction_marshalling.go#L102)
  * In Solana, it can be used as `json.Unmarshal(txBytes, &t)`.
* **In the following points, assume that `node` is from a node connection in the first bullet.**
* **node.BlockByNumber**. Seems self explanatory for what it needs to do.
  * [Ethereum](https://github.com/ethereum/go-ethereum/blob/604e215d1bb070dff98fb76aa965064c74e3633f/ethclient/ethclient.go#L86):
  	```go
	block, err := node.BlockByNumber(context.Background(), index)
	```
  * Solana doesn't use this, but it ignores the number and seems to get the latest block as type `*rpc.GetBlockResult` with [this](https://github.com/gagliardetto/solana-go/blob/290a21adc5d262d93baba0378ebf1dc9a5a1d21d/rpc/getBlock.go#L82)
  	```go
	block, err = node.GetBlockWithOpts(..params here..)
	```
* **block.Transactions**. This can be of any form, but we need some way to get a list of transactions from a block.
  * [Ethereum](https://github.com/ethereum/go-ethereum/blob/604e215d1bb070dff98fb76aa965064c74e3633f/core/types/block.go#L316): `for _, v := range block.Transactions()` and then uses `v.Hash().String()` to represent the transaction for the rest of the time.
  * Solana: `for _, sig := range block.Signatures`
    * Cannot find code for Solana right now
* **node.Subscribe**. We need to subscribe get notifications from a node. In this case, I think we hear when it has a new block.
  * [Ethereum](https://github.com/ethereum/go-ethereum/blob/604e215d1bb070dff98fb76aa965064c74e3633f/ethclient/ethclient.go#L322):
  	```go
	eventCh := make(chan *ethtypes.Header)
	sub, err := node.SubscribeNewHead(context.Background(), eventCh)
	```
    * The `sub` is a subscription, and eventually we need to call `sub.Unsubscribe()`.
  * [Solana](https://github.com/gagliardetto/solana-go/blob/290a21adc5d262d93baba0378ebf1dc9a5a1d21d/rpc/ws/rootSubscribe.go#L21): specifically for a node of type `ws.Client`
  	```go
	sub, err := node.RootSubscribe()
	```
    * This also has `sub.Unsubscribe()`.
* **node.SendTransaction**. Send a transaction over to a node
  * Ethereum: 
  	```go
	txSigned := tx.(*ethtypes.Transaction)
	err := node.SendTransaction(context.Background(), &txSigned)
	```
	where `tx` is of type `interface{}` and `txSigned` is of type `ethtypes.Transaction`.
  * Solana: `node.SendTransactionWithOpts(..params here..)` where node has type `rpc.Client`.
* **node.Close**. Close a connection with a node. Both Ethereum and Solana use `node.Close()`


Other info needed:
How do we represent a transaction? Use signature? Use hash? Who knows?

**We will probably get these from https://github.com/zcash/zcash/blob/master/src/rpc/**

## Implementation of blockchain_interface.go

In the rest of this markdown file, I paste the code from `ethereum_interface.go` and `solana_interface.go` that implement `blockchain_interface.go`. I have written comments in this code that should help us understand where the two are different and what we need to do to write the Zcash interface.

Here is Ethereum.

```go
/** diablo-benchmark-2\blockchains\clientinterfaces\ethereum_interface.go */
package clientinterfaces

/** my comments in this style */

// This client is based off the examples:
// https://github.com/ethereum/go-ethereum/blob/master/rpc/client_example_test.go

import (
	"context"
	"diablo-benchmark/blockchains/workloadgenerators"
	"diablo-benchmark/core/configs"
	"diablo-benchmark/core/results"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
)

/** This is a chainConfig:

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

/** secondary nodes here is not anything to do with primary/secondary diablo setup.
It is instead the rpc connections I think */
// EthereumInterface is the the Ethereum implementation of the clientinterface
// Provides functionality to interaact with the Ethereum blockchain
type EthereumInterface struct {
	PrimaryNode      *ethclient.Client      // The primary node connected for this client.
	SecondaryNodes   []*ethclient.Client    // The other node information (for secure reads etc.)
	SubscribeDone    chan bool              // Event channel that will unsub from events
	TransactionInfo  map[string][]time.Time // Transaction information
	bigLock          sync.Mutex
	HandlersStarted  bool         // Have the handlers been initiated?
	StartTime        time.Time    // Start time of the benchmark
	ThroughputTicker *time.Ticker // Ticker for throughput (1s)
	Throughputs      []float64    // Throughput over time with 1 second intervals
	GenericInterface
}

// Init initialises the list of nodes
func (e *EthereumInterface) Init(chainConfig *configs.ChainConfig) {
	e.Nodes = chainConfig.Nodes
	e.TransactionInfo = make(map[string][]time.Time, 0)
	e.SubscribeDone = make(chan bool)
	e.HandlersStarted = false
	e.NumTxDone = 0
}

// Cleanup formats results and unsubscribes from the blockchain
func (e *EthereumInterface) Cleanup() results.Results {
	// Stop the ticker
	e.ThroughputTicker.Stop()

	// clean up connections and format results
	if e.HandlersStarted {
		e.SubscribeDone <- true
	}

	txLatencies := make([]float64, 0)
	var avgLatency float64

	var endTime time.Time

	success := uint(0)
	fails := uint(e.Fail)

	/** This ignores the string, but solana uses it: `for sig, v in range e.TransactionInfo {` */
	for _, v := range e.TransactionInfo {
		if len(v) > 1 {
			txLatency := v[1].Sub(v[0]).Milliseconds()
			txLatencies = append(txLatencies, float64(txLatency))
			avgLatency += float64(txLatency)
			if v[1].After(endTime) {
				endTime = v[1]
			}

			success++
		} else {
			fails++
		}
	}

	/** Solana has a logger that it reuses. See Solana for better debugging practices */
	zap.L().Debug("Statistics being returned",
		zap.Uint("success", success),
		zap.Uint("fail", fails))

	// Calculate the throughput and latencies
	var throughput float64
	if len(txLatencies) > 0 {
		throughput = (float64(e.NumTxDone) - float64(e.Fail)) / (endTime.Sub(e.StartTime).Seconds())
		avgLatency = avgLatency / float64(len(txLatencies))
	} else {
		avgLatency = 0
		throughput = 0
	}

	averageThroughput := float64(0)
	var calculatedThroughputSeconds = []float64{e.Throughputs[0]}
	for i := 1; i < len(e.Throughputs); i++ {
		calculatedThroughputSeconds = append(calculatedThroughputSeconds, float64(e.Throughputs[i]-e.Throughputs[i-1]))
		averageThroughput += float64(e.Throughputs[i] - e.Throughputs[i-1])
	}

	averageThroughput = averageThroughput / float64(len(e.Throughputs))

	zap.L().Debug("Results being returned",
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

/** Solana uses a for loop, but the internal code is the same */
// throughputSeconds calculates the throughput over time, to show dynamic
func (e *EthereumInterface) throughputSeconds() {
	e.ThroughputTicker = time.NewTicker((time.Duration(e.Window) * time.Second))
	seconds := float64(0)

	for {
		select {
		case <-e.ThroughputTicker.C:
			seconds += float64(e.Window)
			e.Throughputs = append(e.Throughputs, float64(e.NumTxDone-e.Fail))
		}
	}
}

// Start sets up the start time and starts the periodic checking of the
// throughput.
func (e *EthereumInterface) Start() {
	e.StartTime = time.Now()
	go e.throughputSeconds()
}

/** Aside from the two commented lines, this is the same as Solana */
// ParseWorkload parses the workload and converts into the type for the benchmark.
func (e *EthereumInterface) ParseWorkload(workload workloadgenerators.WorkerThreadWorkload) ([][]interface{}, error) {
	parsedWorkload := make([][]interface{}, 0)

	for _, v := range workload {
		intervalTxs := make([]interface{}, 0)
		for _, txBytes := range v {
			t := ethtypes.Transaction{}     /** This is specific to Ethereum */
			err := t.UnmarshalJSON(txBytes) /** e.g. solana uses json.Unmarshal(txBytes, &t) */
			if err != nil {
				return nil, err
			}

			intervalTxs = append(intervalTxs, &t)
		}
		parsedWorkload = append(parsedWorkload, intervalTxs)
	}

	e.TotalTx = len(parsedWorkload)

	return parsedWorkload, nil
}

// parseBlocksForTransactions parses the the given block number for the transactions
func (e *EthereumInterface) parseBlocksForTransactions(blockNumber *big.Int) {
	block, err := e.PrimaryNode.BlockByNumber(context.Background(), blockNumber)

	if err != nil {
		zap.L().Warn(err.Error())
		return
	}

	tNow := time.Now()
	var tAdd uint64

	e.bigLock.Lock()

	/** Solana ranges over block signatures */
	for _, v := range block.Transactions() {
		tHash := v.Hash().String()
		if _, ok := e.TransactionInfo[tHash]; ok {
			e.TransactionInfo[tHash] = append(e.TransactionInfo[tHash], tNow)
			tAdd++
		}
	}

	e.bigLock.Unlock()

	atomic.AddUint64(&e.NumTxDone, tAdd)
}

// EventHandler subscribes to the blocks and handles the incoming information about the transactions
func (e *EthereumInterface) EventHandler() {
	// Channel for the events
	eventCh := make(chan *ethtypes.Header)

	sub, err := e.PrimaryNode.SubscribeNewHead(context.Background(), eventCh) /** This subsciption is unique by blockchain */
	if err != nil {
		zap.Error(err)
		return
	}

	for {
		select { /** Need to understand this better, but it seems like we iterate through the subscriptions, and if done, we quit. Meanwhile, update transaction counts */
		case <-e.SubscribeDone:
			sub.Unsubscribe()
			return
		case header := <-eventCh:
			// Got a head
			go e.parseBlocksForTransactions(header.Number) /** count the number of blocks in the block and add to "self.counter" */
		case err := <-sub.Err():
			zap.L().Warn(err.Error())
		}
	}
}

// ParseBlocksForTransactions Goes through all the blocks between start and end index, and check for the
// transactions contained in the blocks. This can help with (A) latency, and
// (B) correctness to ensure that committed transactions are actually in the blocks.
func (e *EthereumInterface) ParseBlocksForTransactions(startNumber uint64, endNumber uint64) error {
	for i := startNumber; i <= endNumber; i++ {
		b, err := e.GetBlockByNumber(i)

		if err != nil {
			return err
		}

		e.bigLock.Lock()

		for _, v := range b.TransactionHashes {
			if _, ok := e.TransactionInfo[v]; ok {
				e.TransactionInfo[v] = append(e.TransactionInfo[v], time.Unix(int64(b.Timestamp), 0))
			}
		}

		e.bigLock.Unlock()
	}

	return nil
}

/** id argument seems to be just an index in the node list */
/** this also starts the eventhandler, which processes blocks and counts transactions I think */
// ConnectOne connects to one node with the node index matching the "ID".
func (e *EthereumInterface) ConnectOne(id int) error {
	// If our ID is greater than the nodes we know, there's a problem!

	if id >= len(e.Nodes) {
		return errors.New("invalid client ID")
	}

	// Connect to the node
	c, err := ethclient.Dial(fmt.Sprintf("ws://%s", e.Nodes[id]))

	// If there's an error, raise it.
	if err != nil {
		return err
	}

	e.PrimaryNode = c

	if !e.HandlersStarted {
		go e.EventHandler()
		e.HandlersStarted = true
	}

	return nil
}

// ConnectAll connects to all nodes given in the hosts
func (e *EthereumInterface) ConnectAll(primaryID int) error {
	// If our ID is greater than the nodes we know, there's a problem!
	if primaryID >= len(e.Nodes) {
		return errors.New("invalid client primary ID")
	}

	// primary connect
	err := e.ConnectOne(primaryID)

	if err != nil {
		return err
	}

	// Connect all the others
	for idx, node := range e.Nodes { /** e.Nodes is of type []string */
		if idx != primaryID {
			c, err := ethclient.Dial(fmt.Sprintf("ws://%s", node))
			if err != nil {
				return err
			}

			e.SecondaryNodes = append(e.SecondaryNodes, c)
		}
	}

	return nil
}

// DeploySmartContract will deploy the transaction and wait for the contract address to be returned.
func (e *EthereumInterface) DeploySmartContract(tx interface{}) (interface{}, error) {
	txSigned := tx.(*ethtypes.Transaction)
	timeoutCTX, _ := context.WithTimeout(context.Background(), 5*time.Second)

	err := e.PrimaryNode.SendTransaction(timeoutCTX, txSigned)

	if err != nil {
		return nil, err
	}

	// TODO: fix to wait for deploy - look at workloadGenerator!
	// Wait for transaction receipt
	r, err := e.PrimaryNode.TransactionReceipt(context.Background(), txSigned.Hash())

	if err != nil {
		return nil, err
	}

	return r.ContractAddress, nil
}

func (e *EthereumInterface) _sendTx(txSigned ethtypes.Transaction) {
	// timoutCTX, _ := context.WithTimeout(context.Background(), 5*time.Second)

	/** utilizes rpc calls */
	/** Where does context come from? */
	err := e.PrimaryNode.SendTransaction(context.Background(), &txSigned)

	// The transaction failed - this could be if it was reproposed, or, just failed.
	// We need to make sure that if it was re-proposed it doesn't count as a "success" on this node.
	if err != nil {
		zap.L().Debug("Err",
			zap.Error(err),
		)
		atomic.AddUint64(&e.Fail, 1)
		atomic.AddUint64(&e.NumTxDone, 1)
	}

	e.bigLock.Lock()
	e.TransactionInfo[txSigned.Hash().String()] = []time.Time{time.Now()} /** simply state that the transaction is being sent right now */
	e.bigLock.Unlock()

	atomic.AddUint64(&e.NumTxSent, 1)
}

// SendRawTransaction sends a raw transaction to the blockchain node.
// It assumes that the transaction is the correct type
// and has already been signed and is ready to send into the network.
func (e *EthereumInterface) SendRawTransaction(tx interface{}) error {
	// NOTE: type conversion might be slow, there might be a better way to send this.
	txSigned := tx.(*ethtypes.Transaction) /** Need to look into the interface{} type to understand this */
	go e._sendTx(*txSigned)

	return nil /** why always return nil? does the goroutine possibly override the return with an error? */
}

// SecureRead will implement a "secure read" - will read a value from all connected nodes to ensure that the
// value is the same.
func (e *EthereumInterface) SecureRead(callFunc string, callPrams []byte) (interface{}, error) {
	// TODO implement
	return nil, nil
}

// GetBlockByNumber will request the block information by passing it the height number.
func (e *EthereumInterface) GetBlockByNumber(index uint64) (block GenericBlock, error error) {

	var ethBlock map[string]interface{}
	var txList []string

	bigIndex := big.NewInt(0).SetUint64(index)

	/** again, where does context come from? */
	b, err := e.PrimaryNode.BlockByNumber(context.Background(), bigIndex)

	if err != nil {
		return GenericBlock{}, err
	}

	if &ethBlock == nil {
		return GenericBlock{}, errors.New("nil block returned")
	}

	for _, v := range b.Transactions() {
		txList = append(txList, v.Hash().String())
	}

	return GenericBlock{
		Hash:              b.Hash().String(),
		Index:             b.NumberU64(),
		Timestamp:         b.Time(),
		TransactionNumber: b.Transactions().Len(),
		TransactionHashes: txList, /** These transactions are appended together above */
	}, nil
}

// GetBlockHeight will get the block height through the RPC interaction. Should return the index
// of the block.
func (e *EthereumInterface) GetBlockHeight() (uint64, error) {

	h, err := e.PrimaryNode.HeaderByNumber(context.Background(), nil)

	if err != nil {
		return 0, err
	}

	return h.Number.Uint64(), nil
}

// Close all the client connections
func (e *EthereumInterface) Close() {
	// Close the main client connection
	e.PrimaryNode.Close()

	// Close all other connections
	for _, client := range e.SecondaryNodes {
		client.Close()
	}
}

```

And here is Solana:

```go
/** diablo-benchmark-2\blockchains\clientinterfaces\solana_interface.go */
package clientinterfaces

/** my comments in this style */

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

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"go.uber.org/zap"
)

type solanaClient struct {
	rpcClient *rpc.Client
	wsClient  *ws.Client
}

/** Ethereum doesn't have the zap. Not sure why this does. It seems to be for debugging */
type SolanaInterface struct {
	Connections      []*solanaClient // Active connections to a blockchain node for information
	NextConnection   uint64
	SubscribeDone    chan bool                        // Event channel that will unsub from events
	TransactionInfo  map[solana.Signature][]time.Time // Transaction information
	bigLock          sync.Mutex
	HandlersStarted  bool         // Have the handlers been initiated?
	StartTime        time.Time    // Start time of the benchmark
	ThroughputTicker *time.Ticker // Ticker for throughput (1s)
	Throughputs      []float64    // Throughput over time with 1 second intervals
	logger           *zap.Logger
	GenericInterface
}

/** Just gets any connection, whereas Ethereum resorts to primary I think */
func (s *SolanaInterface) ActiveConn() *solanaClient {
	i := atomic.AddUint64(&s.NextConnection, 1)
	client := s.Connections[i%uint64(len(s.Connections))]
	return client
}

/** Only needed because of logger I think */
func NewSolanaInterface() *SolanaInterface {
	return &SolanaInterface{logger: zap.L().Named("SolanaInterface")}
}

/** This differs from Ethereum only in transaction info. We need to see why transactions are represented differently */
func (s *SolanaInterface) Init(chainConfig *configs.ChainConfig) {
	s.logger.Debug("Init")
	s.Nodes = chainConfig.Nodes
	s.TransactionInfo = make(map[solana.Signature][]time.Time, 0)
	s.SubscribeDone = make(chan bool)
	s.HandlersStarted = false
	s.NumTxDone = 0
}

func (s *SolanaInterface) Cleanup() results.Results {
	s.logger.Debug("Cleanup")
	// Stop the ticker
	s.ThroughputTicker.Stop()

	// clean up connections and format results
	if s.HandlersStarted {
		s.SubscribeDone <- true
	}

	txLatencies := make([]float64, 0)
	var avgLatency float64

	var endTime time.Time

	success := uint(0)
	fails := uint(s.Fail)

	for sig, v := range s.TransactionInfo {
		if len(v) > 1 {
			txLatency := v[1].Sub(v[0]).Milliseconds()
			txLatencies = append(txLatencies, float64(txLatency))
			avgLatency += float64(txLatency)
			if v[1].After(endTime) {
				endTime = v[1]
			}

			success++
		} else {
			/** Here's what I think this is doing: If the transaction was not processed twice (i.e. it was only */
			/** sent out and never verified or put onto the blockchain, then it's a fail. */
			/** Then do some error logging based on the failed transaction. */
			/** This is the only real difference from Ethereum in the Init function */
			s.logger.Debug("Missing", zap.String("sig", sig.String()))
			status, err := s.ActiveConn().rpcClient.GetSignatureStatuses(context.Background(), true, sig)
			if err != nil {
				s.logger.Debug("Status", zap.Error(err))
			} else {
				s.logger.Debug("Status", zap.Any("status", status.Value))
			}
			fails++
		}
	}

	s.logger.Debug("Statistics being returned",
		zap.Uint("success", success),
		zap.Uint("fail", fails))

	// Calculate the throughput and latencies
	var throughput float64
	if len(txLatencies) > 0 {
		throughput = (float64(s.NumTxDone) - float64(s.Fail)) / (endTime.Sub(s.StartTime).Seconds())
		avgLatency = avgLatency / float64(len(txLatencies))
	} else {
		avgLatency = 0
		throughput = 0
	}

	averageThroughput := float64(0)
	var calculatedThroughputSeconds = []float64{s.Throughputs[0]}
	for i := 1; i < len(s.Throughputs); i++ {
		calculatedThroughputSeconds = append(calculatedThroughputSeconds, float64(s.Throughputs[i]-s.Throughputs[i-1]))
		averageThroughput += float64(s.Throughputs[i] - s.Throughputs[i-1])
	}

	averageThroughput = averageThroughput / float64(len(s.Throughputs))

	s.logger.Debug("Results being returned",
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

/** Ethereum uses for select case, but the internal code is the same */
func (s *SolanaInterface) throughputSeconds() {
	s.ThroughputTicker = time.NewTicker((time.Duration(s.Window) * time.Second))
	seconds := float64(0)

	for range s.ThroughputTicker.C {
		seconds += float64(s.Window)
		s.Throughputs = append(s.Throughputs, float64(s.NumTxDone-s.Fail))
	}
}

func (s *SolanaInterface) Start() {
	s.logger.Debug("Start")
	s.StartTime = time.Now()
	go s.throughputSeconds()
}

/** Only the t := .. and err := ... lines are unique for each blockchain */
func (s *SolanaInterface) ParseWorkload(workload workloadgenerators.WorkerThreadWorkload) ([][]interface{}, error) {
	s.logger.Debug("ParseWorkload")
	parsedWorkload := make([][]interface{}, 0)

	for _, v := range workload {
		intervalTxs := make([]interface{}, 0)
		for _, txBytes := range v {
			t := solana.Transaction{}
			err := json.Unmarshal(txBytes, &t)
			if err != nil {
				return nil, err
			}

			intervalTxs = append(intervalTxs, &t)
		}
		parsedWorkload = append(parsedWorkload, intervalTxs)
	}

	s.TotalTx = len(parsedWorkload)

	return parsedWorkload, nil
}

// parseBlocksForTransactions parses the the given block number for the transactions
func (s *SolanaInterface) parseBlocksForTransactions(slot uint64) {
	s.logger.Debug("parseBlocksForTransactions", zap.Uint64("slot", slot))

	/** This part is very different from Ethereum. Ethereum just requests from PrimaryNode and quits if it fails */
	var block *rpc.GetBlockResult
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

	tNow := time.Now()
	var tAdd uint64 /** basically we just need to count the number of valid transactions */

	s.bigLock.Lock()

	/** Ethereum ranges over block transactions */
	for _, sig := range block.Signatures {
		/** This is custom based on transaction info */
		if info, ok := s.TransactionInfo[sig]; ok && len(info) == 1 {
			s.TransactionInfo[sig] = append(info, tNow)
			tAdd++
		}
	}

	s.bigLock.Unlock()

	atomic.AddUint64(&s.NumTxDone, tAdd)
	s.logger.Debug("Stats", zap.Uint64("sent", atomic.LoadUint64(&s.NumTxSent)), zap.Uint64("done", atomic.LoadUint64(&s.NumTxDone)))
}

// EventHandler subscribes to the blocks and handles the incoming information about the transactions
func (s *SolanaInterface) EventHandler() {
	s.logger.Debug("EventHandler")
	sub, err := s.ActiveConn().wsClient.RootSubscribe() /** What does it mean to subscribe to a connection? */
	if err != nil {
		s.logger.Warn("RootSubscribe", zap.Error(err))
		return
	}
	defer sub.Unsubscribe()
	go func() {
		for range s.SubscribeDone { /** Does the body only trigger if SubscribeDone is nonempty? */
			sub.Unsubscribe()
			return
		}
	}()

	var currentSlot uint64 = 0
	for { /** ~= while true */
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

func (s *SolanaInterface) ConnectOne(id int) error {
	s.logger.Debug("ConnectOne")
	return errors.New("do not use")
}

func (s *SolanaInterface) ConnectAll(primaryID int) error {
	s.logger.Debug("ConnectAll")
	// If our ID is greater than the nodes we know, there's a problem!
	if primaryID >= len(s.Nodes) {
		return errors.New("invalid client primary ID")
	}

	/** Seems to ignore the primary connect */

	// Connect all the others
	for _, node := range s.Nodes {
		/** This loop body is very dependent on rpc */
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

/** Why is this not implemented? Is it not needed? */
func (s *SolanaInterface) DeploySmartContract(tx interface{}) (interface{}, error) {
	s.logger.Debug("DeploySmartContract")
	return nil, errors.New("not implemented")
}

func (s *SolanaInterface) SendRawTransaction(tx interface{}) error {
	go func() { /** Ethereum also runs this as a goroutine */
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

/** Ethereum also does not implement this */
func (s *SolanaInterface) SecureRead(callFunc string, callParams []byte) (interface{}, error) {
	s.logger.Debug("SecureRead")
	return nil, errors.New("not implemented")
}

/** Ethereum implements all of the following */
func (s *SolanaInterface) GetBlockByNumber(index uint64) (GenericBlock, error) {
	s.logger.Debug("GetBlockByNumber")
	return GenericBlock{}, errors.New("not implemented")
}

func (s *SolanaInterface) GetBlockHeight() (uint64, error) {
	s.logger.Debug("GetBlockHeight")
	return 0, errors.New("not implemented")
}

/** is overloaded from above */
func (s *SolanaInterface) ParseBlocksForTransactions(startNumber uint64, endNumber uint64) error {
	s.logger.Debug("ParseBlocksForTransactions")
	return errors.New("not implemented")
}

func (s *SolanaInterface) Close() {
	s.logger.Debug("Close")
	// Close all connections
	for _, client := range s.Connections {
		client.wsClient.Close()
	}
}
```