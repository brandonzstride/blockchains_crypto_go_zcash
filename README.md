# Running Zcash in Diablo
Brandon Stride, Robert Zhang, Elisée Djapa  | Blockchains & Cryptocurrencies Spring 23

## Problem statement
We want to benchmark the Zcash cryptocurrency in a standardized way. The Diablo benchmarking application has been used already to test Ethereum, Solana, Diem, Quorum, Algorand, and Avalanche. Let's add Zcash to this list.

Read a little bit about Diablo here: https://diablobench.github.io/

And access the latest Diablo paper here: https://www.researchgate.net/publication/367219444_Diablo_A_Benchmark_Suite_for_Blockchains

Check out the Zcash github here: https://github.com/zcash/zcash

And the first Zcash paper here: http://zerocash-project.org/media/pdf/zerocash-oakland2014.pdf

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

2. **SWIG minimal example**; Fill in details here on running SWIG with ``hello world'' files.

    https://go.dev/doc/go1.2#cgo_and_cpp

