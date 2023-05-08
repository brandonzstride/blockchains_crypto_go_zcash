# Automatically flatten files nested directories

This is a tool to extract specified files from nested directories into a single
directory, mainly to make way for easy application of SWIG on wrapping C++
programs in Go since SWIG doesn't support nested directories.

## Usage

```sh
dune exec -- src/cpcpp.exe -worklist examples/example1/spec1.json
# where spec1.json is a worklist file

# or, to call the executable directly,
./cpcpp.exe -worklist examples/example1/spec1.json
```

Note that the executable has be built first, or else it can only be run on the
system where it was last built.

## Specification file format

```json
{
  "source": "the root of the nested directories",
  "target": "the target directory to dump all the files into",
  "worklist": {
    "dir": "a subdirectory",
    "files": [
      "a.cpp",
      "b.cpp",
      {
        "dir": "a subsubdirectory",
        "files": [
          "c.cpp",
          "d.cpp"
        ]
      }
    ]
  }
}
```

It's surprisingly hard to achieve this concise format AND be able to cast it
into OCaml types. Spent quite a bit of time on this but eventually got it to
work!
