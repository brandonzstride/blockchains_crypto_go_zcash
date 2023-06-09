package blockchain


import (
	"diablo-benchmark/core/configs"
	"diablo-benchmark/core/results"
	"diablo-benchmark/core/workload"
)


// This runs on the primary node.
// Its goal is to prepare the blockchain for the incoming benchmark by putting
// it in a desired state and by planing the workload to send during the
// benchmark.
//
type Controller interface {
	// Initialize this Controller with the configuration of the current
	// blockchain and the incoming benchmark.
	//
	Init(c *configs.ChainConfig, b *configs.BenchConfig, t []int) error

	// Setup the blockchain state as needed by the benchmark.
	// This is typically used to deploy contracts and create assets.
	//
	Setup() error

	// Generate the benchmark workload.
	//
	// This is the ideal spot to define what blockchain node each
	// secondary/thread must contact, which transactions to send etc...
	// since the controller has a global view of the system.
	//
	// The workload is defined as:
	//   - for each secondary node
	//     - for each worker thread
	//       - for each time interval
	//         - a list of transaction
	// Each transaction is an opaque slice of bytes.
	// This controller can add transactions by calling:
	//
	//     workloads.Workload.Add(secondaryId, threadId, interval, tx)
	//
	// The secondaryId, threadId and interval are in the ranges:
	//   - secondaryId: [ 0 ; config.Secondaries )
	//   - threadId:    [ 0 ; config.Threads )
	//   - interval:    txs
	// where config is the "b" parameter of Init() and txs is the "t"
	// parameter of Init
	//
	Generate() (*workload.Workload, error)
}


// This runs on each secondary, once per worker thread.
// Its goal is to execute the benchmark workload and collect results as they
// would be seen by an actual client.
//
type Worker interface {
	// Initialize this Worker with the configuration of the current 
	// blockchain.
	//
	Init(c *configs.ChainConfig) error

	// Parse the workload generated by Controller.Generate.
	// Each worker sees only its own workload. This is the last place to
	// precompute everything needed before the benchmark actually starts.
	// This Worker stores the parsed transactions as they will be required
	// during the benchmark.
	//
	ParseWorkload(workload [][]byte) error

	// Does everything needed when the benchmark starts.
	// This is typically a sequence of short actions such as storing the
	// current time or launching a few goroutines.
	//
	StartBenchmark() error

	// Send the transaction parsed during ParseWorkload with the given
	// index.
	// This function is called only once with a given index.
	//
	SendTransaction(index int) error

	// Does everything needed when the benchmark stops.
	// This is typically the place to cancel pending transactions and store
	// the current time.
	//
	StopBenchmark() error

	// Generate the results collected during the benchmark.
	//
	Generate() *results.EventLog
}
