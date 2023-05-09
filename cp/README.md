# Automatically flatten files in nested directories

This is a tool to extract specified files from nested directories into a single
directory, mainly to make way for easy application of SWIG on wrapping C++
programs in Go since SWIG doesn't support nested directories.

## Usage

```sh
dune exec -- src/cp.exe -spec examples/example1/spec1.json

# or, to call the executable directly
./cp.exe -spec examples/example1/spec1.json
```

Note that the executable has be built first, or else it can only be run on the
system where it was last built. To install OCaml and required tooling, see
[Install](##Install).

## Specification file format

```json
{
  "source": "the root of the nested directories",
  "target": "the target directory to dump all the files into",
  "swigcxx": "the .swigcxx file to generate, or not if this field is left empty",
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
work!

## Install

Follow the [official instructions](https://ocaml.org/docs/up-and-running) to install opam and OCaml.

```sh
# Install the required packages
opam install dune core core_unix fileutils yojson ppx_yojson_conv ppx_deriving

# Build the executable
dune build
```
