# Running Zcash in Diablo

**Authors:** Brandon Stride, Robert Zhang, Elisée Djapa<br>
*Blockchains & Cryptocurrencies Spring 23*

## Problem Statement
We want to benchmark the Zcash cryptocurrency in a standardized way using the Diablo benchmarking application. Diablo has already been used to test Ethereum, Solana, Diem, Quorum, Algorand, and Avalanche. Our goal is to add Zcash to this list.

- Read more about Diablo [here](https://diablobench.github.io/)
- Access the latest Diablo paper [here](https://www.researchgate.net/publication/367219444_Diablo_A_Benchmark_Suite_for_Blockchains)
- Check out the Zcash GitHub repository [here](https://github.com/zcash/zcash)
- Read the first Zcash paper [here](http://zerocash-project.org/media/pdf/zerocash-oakland2014.pdf)

## Go Zcash
Diablo runs in Go. All of the previously-tested blockchains have a Go implementation or have Go SDKs (e.g. Solana is in Rust). Zcash is written in C++ and C. We need to call Zcash functions from Go.
1. **hello world files**; We began with some ``hello world'' files.
    ```c++
    /* hello.cpp */
    #include <iostream>

    int main() {
        std::cout << "Hello, world in C++!" << std::endl;
        return 0;
    }
    ```

    Run it:
    ```console
    > g++ hello.cpp -o hello
    > ./hello
    Hello, world in C++!
    ```

    And in Go:
    ```go
    /* hello.go */
    package main

    import "fmt"

    func main() {
        fmt.Println("Hello, world in Go!")
    }
    ```

    Run it:
    ```console
    > go run hello.go
    Hello, world in Go!
    ```

    Both of these files run quite easily, but now we want to run `hello.c` from a Go file. According to swig.org, "Go does not support direct calling of functions written in C/C++. The cgo program may be used to generate wrappers to call C code from Go, but there is no convenient way to call C++ code. SWIG fills this gap".

----
---

2. **SWIG minimal example**; Fill in details here on running SWIG with ``hello world'' files.

    For more details, refer to the following resource: [Go and C++](https://go.dev/doc/go1.2#cgo_and_cpp)

---
---
# Zcash Blockchain Implementation Overview

This is  a high-level overview of the Zcash blockchain implementation, describing important components and their respective locations in the codebase. [Check out the ZCash Repo Link Here for more details](https://github.com/zcash/zcash)

## Consensus Rules

The consensus rules define how nodes agree on the blockchain's state. Zcash's consensus rules are implemented in the `src/consensus/` directory. Key files include:

- `params.h`: Parameters for consensus rules
- `consensus.cpp`: Implements consensus-critical validation
- `upgrades.cpp`: Handles network upgrades

## Transaction and Block Validation

The `src/` directory contains implementations for transaction and block validation:

- `main.cpp`: Functions related to block validation and chain management
- `txmempool.cpp`: Responsible for transaction memory pool management

## zk-SNARKs and Zero-Knowledge Proofs

The implementation of zk-SNARKs can be found in the `src/zcash/` directory. Notable files include:

- `JoinSplit.hpp`: Defines the JoinSplit proof system
- `Proof.hpp`: Represents a zk-SNARK proof
- `NoteEncryption.hpp`: Implements the encryption scheme used for shielded transactions

## Wallet

The wallet code is located in the `src/wallet/` directory:

- `wallet.cpp` and `wallet.h`: Define the core wallet functionality, including key generation, transaction creation, and transaction signing

## P2P Network

The peer-to-peer network code is in the `src/net.*` files, handling node connections, message exchanges, and block propagation.

## RPC (Remote Procedure Call) Interface

The RPC interface allows developers to interact with the Zcash node programmatically. The `src/rpc/` directory contains files for various RPC calls:

- `blockchain.cpp`: Provides blockchain-related RPC calls
- `wallet.cpp`: Provides wallet-related RPC calls

## Mining

The mining process in Zcash is implemented in the `src/miner.*` files, managing the creation of new blocks and the integration of transactions into those blocks.
