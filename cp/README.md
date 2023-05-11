# Automatically flatten files in nested directories

This is a tool to extract specified files from nested directories into a single
directory, mainly to make way for easy application of SWIG on wrapping C++
programs in Go since SWIG doesn't support nested directories.

See the [zcash_rpc](zcash_rpc) for a concrete example of using `cp` to port C++
code to Go.

## Usage

```sh
dune exec -- src/cp.exe -spec examples/example1/spec1.json

# or, to call the executable directly
./dist/mac_m1/cp.exe -spec examples/example1/spec1.json

# to build and copy the executable to the `zcash_rpc` directory
make
```

We provide pre-built executables for Mac, M1 Mac, and Ubuntu in the
[dist](./dist) folder. However, you may still need to build the project to
generate an executable runnable on your machine. To install OCaml and required
tooling, see [Install](##Install).

## Specification file format

```json
{
  "source": "the root of the nested directories",
  "target": "the target directory to dump all the files into",
  "swig": true, // whether to generate SWIG-related files
  "worklist": {
    "dir": "a subdirectory",
    "files": [
      "a.cpp",
      "#b.cpp", // '#' at the beginning adds a corresponding include statement in the generated .swigcxx file
      {
        "dir": "a subsubdirectory",
        "files": [
          "#c.cpp",
          "#d.cpp"
        ]
      }
    ]
  }
}
```

It's surprisingly hard to achieve this concise format AND be able to cast it
into OCaml types. Spent quite a bit of time on this but eventually got it to
work! See our final report for details.

Note that one needs to manually add a pair of `#include` and `%include`
statements for headers that are not copied files in the generated `.swigcxx`
file, like for `iostream`. See our final report for details on how the
`.swigcxx` file is composed.

## Install

Follow the [official instructions](https://ocaml.org/docs/up-and-running) to install opam and OCaml.

```sh
# Install the required packages
opam install dune core core_unix fileutils yojson ppx_yojson_conv ppx_deriving

# Build the executable
dune build
```
