open Core

(* type definitions *)
type file_or_dir =
  (* file to copy *)
  | File of string
  (* file to also include in .swigcxx *)
  | IFile of string
  (* subdirectory to copy from *)
  | Dir of directory

and directory = { dir : string; files : file_or_dir list }

and spec = {
  (* root source directory to copy files from *)
  source : string;
  (* target directory to copy files to *)
  target : string;
  (* whether to generate SWIG-related files *)
  swig : bool;
  (* worklist of files and subdirectories to copy *)
  worklist : directory;
}
[@@deriving yojson]

(* mold raw Yojson input into format acceptable to be converted to our OCaml types
   k: last seen key of a record entry
   yojson: input Yojson object *)
let rec mold ?(k = "") (yojson : Yojson.Safe.t) : Yojson.Safe.t =
  let open String in
  match yojson with
  | `List lst -> `List (List.map lst ~f:(fun x -> mold ~k x))
  | `String s when k = "files" ->
      if Char.(s.[0] = '#') then
        `List
          [ `String "IFile"; `String (Stdlib.String.sub s 1 (length s - 1)) ]
      else `List [ `String "File"; yojson ]
  | `Assoc lst when k = "files" ->
      `List
        (`String "Dir"
        :: [ `Assoc (List.map lst ~f:(fun (k, v) -> (k, mold ~k v))) ])
  | `Assoc lst -> `Assoc (List.map lst ~f:(fun (k, v) -> (k, mold ~k v)))
  | _ -> yojson

(* override the auto-generated Yojson to spec function *)
let spec_of_yojson x = spec_of_yojson (mold x)

(* process the JSON specification and perform the file copying operation
   and return a list of file names to be included in .swigxx *)
let rec copy_files_from_spec dir parent_dir target incl =
  let cur_dir = Filename.concat parent_dir dir.dir in
  List.fold ~init:incl dir.files ~f:(fun acc -> function
    | File file ->
        let source = Filename.concat cur_dir file in
        FileUtil.cp [ source ] target;
        acc
    | IFile file ->
        let source = Filename.concat cur_dir file in
        FileUtil.cp [ source ] target;
        file :: acc
    | Dir worklist -> copy_files_from_spec worklist cur_dir target acc)

(* generate a complete .swigcxx file *)
let gen_swigcxx file_path to_include =
  let inner_incls =
    List.map to_include ~f:(fun incl -> "#include \"" ^ incl ^ "\"")
  in
  let outer_incls =
    List.map to_include ~f:(fun incl -> "%include \"" ^ incl ^ "\"")
  in
  (* concat as full list of file lines *)
  let swigcxx_ls =
    ("%module wrapped" :: "%{" :: inner_incls) @ ("%}" :: outer_incls)
  in
  (* write lines to file *)
  Out_channel.write_all
    (Filename.concat file_path "wrapped.swigcxx")
    ~data:(String.concat ~sep:"\n" swigcxx_ls)

(* generate a go package file as the root of our wrapped Go package *)
let gen_go_package file_path =
  Out_channel.write_all
    (Filename.concat file_path "wrapped.go")
    ~data:"package wrapped"

(* generate a go.mod file as the root of our consumer Go package *)
let gen_go_mod file_path =
  Out_channel.write_all
    (Filename.concat file_path "go.mod")
    ~data:(Format.sprintf "module consumer\ngo 1.20")

(* generate a main.go file for our consumer Go package *)
let gen_main_go file_path =
  Out_channel.write_all
    (Filename.concat file_path "main.go")
    ~data:
      (Format.sprintf
         "package main\nimport (\n\"consumer/wrapped\"\n)\nfunc main() {\n}")

(* main program Logic*)
let () =
  let spec_file = ref "" in
  (* read commandline arguments *)
  Arg.parse
    [
      ( "-spec",
        Arg.Set_string spec_file,
        "JSON file specifying which files to copy over." );
    ]
    (fun _ -> ())
    "./cp.exe -spec PATH_TO_JSON_FILE";

  let json_content = Core.In_channel.read_all !spec_file in
  (* parse JSON string into an object of type spec *)
  let spec_obj = spec_of_yojson @@ Yojson.Safe.from_string json_content in

  let source = spec_obj.source in
  let target = spec_obj.target in
  let worklist = spec_obj.worklist in
  let gen_swig = spec_obj.swig in

  (* remove target directory if already exists *)
  (* FileUtil.rm ~recurse:true [ target ]; *)
  Core_unix.mkdir_p target;
  (* directory to contain the Go package for the wrapped definitions *)
  let wrapped_dir = Filename.concat target "wrapped" in
  Core_unix.mkdir_p wrapped_dir;

  let to_include = copy_files_from_spec worklist source wrapped_dir [] in
  (* only generate SWIG-related files if flag is true *)
  if gen_swig then (
    gen_swigcxx wrapped_dir to_include;
    gen_go_package wrapped_dir;
    gen_go_mod target;
    gen_main_go target)
