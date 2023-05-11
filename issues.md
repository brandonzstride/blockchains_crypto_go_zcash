# Issues

This file elaborates on all the issues we encountered while working on this
project (spoiler: there are many!) and what we did to solve/work around them.

## Importing the Zcash codebase

While working with SWIG, a natural question that occurred is how we can
effectively "import" Zcash's C++ code into ours so that we can use SWIG to
process it to be called in Go.

Online SWIG examples unanimously contain C++ code along with SWIG specifications
all in one directory. This is not sustainable for us because we definitely don't
want to copy all the relevant Zcash files and those they depend on into our own
directory.

The problem is, using `#include` statements to link code from subdirectories
is NOT supported by SWIG, leading to obscure errors related to SWIG. Please
refer to the [failing_swig_example](failing_swig_example) folder for this
failing setup. In fact, official docs indicates that one should "[put] other
C/C++ code in the same directory" as the SWIG specification file.

## Creating a tool to automatically flatten files in nested directories

As a result of SWIG requiring all C++ source files to be in the same directory,
we decided to write a tool to automate copying files from a big project like
Zcash.

We hope this effort, alongside exploring this barely mentioned drawback of SWIG,
will be of use to others.

Please refer to the [cp/README.md](cp/README.md) folder for the details and
difficulties we faced.

## Using the Zcash RPC

The Zcash RPC is part of Zcash's CLI operating over the full node. It allows us
to query on different stats of the blockchain, such as the current block count.
We thought of making use of the RPC instead of using SWIG to port C++ code to
Go, but the prerequisite to the RPC working is to download and build the entire
Zcash blockchain, which is a very resource- and time-consuming process. As a
result, we decided to explore options with SWIG while trying to download the
full node on and off on the side (because we wanted to still use our laptops for
other purposes; the download process has been taking a toll on our machines).

## Better idea may be to benchmark using executables instead of source code

The above issues prompted us to think of a potentially better way to benchmark
blockchains - calling blockchain primitives via compiled executables instead of
the source code (which often is written in a different programming language from
that of the benchmarking code). This way, we can avoid the hassle of porting
foreign code and (maybe even) avoid downloading the full blockchain. Much work
can be dedicated to reasoning about exactly this can be carried out and
streamlined, which we unfortunataly don't have the time for.
