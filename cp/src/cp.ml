(* open Core

type file_or_dir = File of string | Dir of dir
and dir = { dir : string; files : file_or_dir list }

and worklist = { source : string; target : string; worklist : dir }
[@@deriving yojson]

let rec mold ?(k = "") (yojson : Yojson.Safe.t) : Yojson.Safe.t =
  let open String in
  match yojson with
  | `List l -> `List (List.map l ~f:(fun x -> mold ~k x))
  | `String _ as x when k = "files" -> `List (`String "File" :: [ x ])
  | `Assoc l when k = "files" ->
      `List
        (`String "Dir"
        :: [ `Assoc (List.map l ~f:(fun (k, v) -> (k, mold ~k v))) ])
  | `Assoc l -> `Assoc (List.map l ~f:(fun (k, v) -> (k, mold ~k v)))
  | _ -> yojson

let worklist_of_yojson x = worklist_of_yojson (mold x)

let rec parse_json dir parent_dir target =
  let cur_dir = Filename.concat parent_dir dir.dir in
  List.iter dir.files ~f:(function
    | File file ->
        let source = Filename.concat cur_dir file in
        FileUtil.cp [ source ] target
    | Dir worklist -> parse_json worklist cur_dir target)

let () =
  let worklist = ref "" in
  Arg.parse
    [
      ( "-worklist",
        Arg.Set_string worklist,
        "JSON specifying which files to copy over." );
    ]
    (fun _ -> ())
    "./cpcpp.exe -worklist PATH_TO_JSON_FILE";
  (* print_endline @@ Safe.show @@ mold @@ Safe.from_string
     @@ Core.In_channel.read_all !worklist; *)
  let obj =
    worklist_of_yojson @@ Yojson.Safe.from_string
    @@ Core.In_channel.read_all !worklist
  in
  let source = obj.source in
  let target = obj.target in
  Core_unix.mkdir_p target;
  let worklist = obj.worklist in
  parse_json worklist source target
 *)




open Core

(* Type definitions *)
type file_or_dir = File of string | Dir of directory
and directory = { dir : string; files : file_or_dir list }
and worklist = { source : string; target : string; worklist : directory }
[@@deriving yojson]

(* Helper function to mold JSON input into the desired OCaml data structure *)
let rec mold_json ?(key = "") (json : Yojson.Safe.t) : Yojson.Safe.t =
  let open String in
  match json with
  | `List lst -> `List (List.map lst ~f:(fun x -> mold_json ~key:key x))
  | `String _ as x when key = "files" -> `List (`String "File" :: [ x ])
  | `Assoc lst when key = "files" ->
      `List
        (`String "Dir"
        :: [ `Assoc (List.map lst ~f:(fun (k, v) -> (k, mold_json ~key:k v))) ])
  | `Assoc lst -> `Assoc (List.map lst ~f:(fun (k, v) -> (k, mold_json ~key:k v)))
  | _ -> json

(* Convert the molded JSON into a worklist data structure *)
let worklist_of_yojson x = worklist_of_yojson (mold_json x)

(* Process the JSON specification and perform the file copying operation *)
let rec copy_files_from_spec dir parent_dir target =
  let cur_dir = Filename.concat parent_dir dir.dir in
  List.iter dir.files ~f:(function
    | File file ->
        let source = Filename.concat cur_dir file in
        FileUtil.cp [ source ] target
    | Dir worklist -> copy_files_from_spec worklist cur_dir target)

(* Main program Logic*)
let () =
  let worklist_file = ref "" in
  Arg.parse
    [
      ( "-worklist",
        Arg.Set_string worklist_file,
        "JSON file specifying which files to copy over." );
    ]
    (fun _ -> ())
    "./cpcpp.exe -worklist PATH_TO_JSON_FILE";
  
  let json_content = Core.In_channel.read_all !worklist_file in
  let worklist_obj = worklist_of_yojson @@ Yojson.Safe.from_string json_content in
  
  let source = worklist_obj.source in
  let target = worklist_obj.target in
  
  Core_unix.mkdir_p target;
  let worklist = worklist_obj.worklist in
  copy_files_from_spec worklist source target




