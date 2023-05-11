# Required RPC calls

Assume the following Ethereum imports:
* "github.com/ethereum/go-ethereum/ethclient" ~= `ethclient`
* "github.com/ethereum/go-ethereum/core/types" ~= `ethtypes`

Assume the following Solana imports:
* "github.com/gagliardetto/solana-go/" ~= `solana`
* "github.com/gagliardetto/solana-go/rpc/" ~= `rpc`
* "github.com/gagliardetto/solana-go/rpc/ws/" ~= `ws`

Click on the hyperlinks in the following sections to see the GitHub code for each RPC call.

## node connection
For the rest of this file, assume that `node` is from one of these connections.

[Ethereum](https://github.com/ethereum/go-ethereum/blob/604e215d1bb070dff98fb76aa965064c74e3633f/ethclient/ethclient.go#L40): connection is achieved via 
```go
c, err := ethclient.Dial(fmt.Sprintf("ws://%s", e.Nodes[id]))
```
where `c` is of type `*ethclient.Client`, and `e.Nodes[id]` is string to identify a node.

[Solana](https://github.com/gagliardetto/solana-go/blob/290a21adc5d262d93baba0378ebf1dc9a5a1d21d/rpc/client.go#L48): connection is achieved via
```go
conn := rpc.New(fmt.Sprintf("http://%s", node))
```
where `node` is a string. It then uses a socket and `ws.Connect` function. Then `conn` is of type `rpc.Client` or `ws.Client`.

## Transaction{}
This is the transaction interface, where `txBytes` from the workload is then loaded into the interface.

In [Ethereum](https://github.com/ethereum/go-ethereum/blob/604e215d1bb070dff98fb76aa965064c74e3633f/core/types/transaction.go#L52), this is 
```go
t := ethtypes.Transaction{}
err := t.UnmarshalJSON(txBytes)
```
where the `UnmarshalJSON` code is [here](https://github.com/ethereum/go-ethereum/blob/604e215d1bb070dff98fb76aa965064c74e3633f/core/types/transaction_marshalling.go#L102)

In [Solana](https://github.com/gagliardetto/solana-go/blob/290a21adc5d262d93baba0378ebf1dc9a5a1d21d/transaction.go#L34), this is
```go
t := solana.Transaction{}
err := json.Unmarshal(txBytes, &t) /** from import encoding/json */
```

## node.BlockByNumber

[Ethereum](https://github.com/ethereum/go-ethereum/blob/604e215d1bb070dff98fb76aa965064c74e3633f/ethclient/ethclient.go#L86):
```go
block, err := node.BlockByNumber(context.Background(), index)
```

Solana doesn't have this exactly, but it ignores the number and seems to get the latest block as type `*rpc.GetBlockResult` [here](https://github.com/gagliardetto/solana-go/blob/290a21adc5d262d93baba0378ebf1dc9a5a1d21d/rpc/getBlock.go#L82)
```go
block, err = node.GetBlockWithOpts(..params here..) /** See solana_interface.go for long list of params */
```

## block.Transactions
This can be of any form, but we need some way to get a list of transactions from a block.

[Ethereum](https://github.com/ethereum/go-ethereum/blob/604e215d1bb070dff98fb76aa965064c74e3633f/core/types/block.go#L316):
```go
for _, v := range block.Transactions() {
    .. /** some code */
    tHash := v.Hash().String() /** use this to represent the transaction from here on */
    .. /** some code */
}
```

Solana:
```go
for _, sig := range block.Signatures {
    /** TODO: fill this in */
}
```
(Cannot find code on GitHub for Solana)

## node.Subscribe

We need to subscribe get notifications from a node. In our specific case, we hear when it has a new block.

[Ethereum](https://github.com/ethereum/go-ethereum/blob/604e215d1bb070dff98fb76aa965064c74e3633f/ethclient/ethclient.go#L322):
```go
eventCh := make(chan *ethtypes.Header)
sub, err := node.SubscribeNewHead(context.Background(), eventCh)
```
where `sub` is a subscription, and eventually we need to call `sub.Unsubscribe()`.

[Solana](https://github.com/gagliardetto/solana-go/blob/290a21adc5d262d93baba0378ebf1dc9a5a1d21d/rpc/ws/rootSubscribe.go#L21): specifically for a node of type `ws.Client`
```go
sub, err := node.RootSubscribe()
```
where `sub` again has `sub.Unsubscribe()`.

## node.SendTransaction**

Send a transaction over to a node.

[Ethereum](https://github.com/ethereum/go-ethereum/blob/604e215d1bb070dff98fb76aa965064c74e3633f/ethclient/ethclient.go#L576): 
```go
txSigned := tx.(*ethtypes.Transaction)
err := node.SendTransaction(context.Background(), &txSigned)
```
where `tx` is of type `interface{}` and `txSigned` is of type `ethtypes.Transaction`.

[Solana](https://github.com/gagliardetto/solana-go/blob/290a21adc5d262d93baba0378ebf1dc9a5a1d21d/rpc/sendTransaction.go#L69):
```go
node.SendTransactionWithOpts(..params here..)
```
where node has type `rpc.Client`.


## node.Close
Close a connection with a node. Both [Ethereum](https://github.com/ethereum/go-ethereum/blob/604e215d1bb070dff98fb76aa965064c74e3633f/ethclient/ethclient.go#L57) and [Solana](https://github.com/gagliardetto/solana-go/blob/290a21adc5d262d93baba0378ebf1dc9a5a1d21d/rpc/client.go#L69) use `node.Close()`