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
  (* location to generate .swigcxx file to *)
  swigcxx : string;
  (* location to generate go package file to *)
  go : string;
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

let get_file_name file_path =
  file_path |> Filename.basename |> Filename.chop_extension

(* generate a complete .swigcxx file *)
let gen_swigcxx file_path to_include =
  let inner_incls =
    List.map to_include ~f:(fun incl -> "#include \"" ^ incl ^ "\"")
  in
  let outer_incls =
    List.map to_include ~f:(fun incl -> "%include \"" ^ incl ^ "\"")
  in
  let file_name = get_file_name file_path in
  (* concat as full list of file lines *)
  let swigcxx_ls =
    (("%module " ^ file_name) :: "%{" :: inner_incls) @ ("%}" :: outer_incls)
  in
  (* write lines to file *)
  Out_channel.write_all file_path ~data:(String.concat ~sep:"\n" swigcxx_ls)

(* generate a go package file as the root of our Go package *)
let gen_go file_path =
  Out_channel.write_all file_path ~data:("package " ^ get_file_name file_path)

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
  Core_unix.mkdir_p target;
  let swigcxx_file_path = spec_obj.swigcxx in
  let go_file_path = spec_obj.go in
  let worklist = spec_obj.worklist in

  let to_include = copy_files_from_spec worklist source target [] in
  (* only generate .swigcxx only if its file path has been specified *)
  if String.(swigcxx_file_path <> "") then
    gen_swigcxx swigcxx_file_path to_include;
  if String.(go_file_path <> "") then gen_go go_file_path
