open Core

(* type definitions *)
type file_or_dir = File of string | IFile of string | Dir of directory
and directory = { dir : string; files : file_or_dir list }

and spec = {
  source : string;
  target : string;
  swigcxx : string;
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

(* process the JSON specification and perform the file copying operation *)
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

(* main program Logic*)
let () =
  let spec_file = ref "" in
  Arg.parse
    [
      ( "-spec",
        Arg.Set_string spec_file,
        "JSON file specifying which files to copy over." );
    ]
    (fun _ -> ())
    "./cp.exe -spec PATH_TO_JSON_FILE";

  let json_content = Core.In_channel.read_all !spec_file in
  let spec_obj = spec_of_yojson @@ Yojson.Safe.from_string json_content in

  let source = spec_obj.source in
  let target = spec_obj.target in
  Core_unix.mkdir_p target;
  let swigcxx_file_path = spec_obj.swigcxx in
  let worklist = spec_obj.worklist in

  let to_include = copy_files_from_spec worklist source target [] in
  (* only generate .swigcxx only if its file path has been specified *)
  if String.(swigcxx_file_path <> "") then
    Swigcxx.gen swigcxx_file_path to_include
