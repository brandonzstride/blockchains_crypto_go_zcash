Here is a working example of using Cgo and Swig to run C++ code from Go.

## Structure and setup

The first step is to package up the C++ code. I will create a folder named `cpp_hello`. This will contain all of the C++ code and the package definitions.
* `hello.h` - header file for some C++ code
* `hello.cpp` - code corresponding to header file above
* `package.go` - defines our Go package that uses the C++ code
* `cpp_hello.swigcxx` - is the Swig file that establishes imports.
  * The stuff inside the "%{ ... %}" is what is required, and below that (i.e. outside the brackets) is what is include in the new package.

---
_NOTES_
* The above files are very short. Read them.
* The header files can be either `.h` or `.hpp` as long as the import and includes are consistent with the file extension.
* All of the following items must have the same name (above: `cpp_hello`)
  * The encompassing folder
  * The `.swigcxx` file
  * The module line in the `.swigcxx` file, e.g. "%module cpp_hello"
  * The package name, i.e. the first line in `package.go`, e.g. "package cpp_hello" 
---

Now that we have a package defined that uses C++ code, we can create a main Go file that uses it. We don't use relative package imports, so we have a file to define the current module. This file is called `go.mod`, and we can create it with the following commands:

```console
> go mod init main_module
> go mod tidy
```

This way, we can import the `cpp_hello` package as `main_module/cpp_hello`. See the code in `main.go` for this (it's very short!).

## Running

Now let's run it:

```console
> go run main.go
Hello, world in C++!
```

The next step is to run C++ that comes deep inside a project.