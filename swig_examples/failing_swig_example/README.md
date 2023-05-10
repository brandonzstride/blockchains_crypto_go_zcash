Command run: `go run main.go`

Error thrown:

```sh
/opt/homebrew/Cellar/go/1.20.3/libexec/pkg/tool/darwin_arm64/link: running c++ failed: exit status 1
Undefined symbols for architecture arm64:
  "print_a()", referenced from:
      __wrap_print_a_cpp_hello_e31752d51a76c271 in 000002.o
      __wrap_print_a_cpp_hello_e31752d51a76c271 in 000003.o
  "print_b()", referenced from:
      __wrap_print_b_cpp_hello_e31752d51a76c271 in 000002.o
      __wrap_print_b_cpp_hello_e31752d51a76c271 in 000003.o
ld: symbol(s) not found for architecture arm64
clang: error: linker command failed with exit code 1 (use -v to see invocation)
```
