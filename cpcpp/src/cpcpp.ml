open Core
open Yojson

type file_or_dir = File of string [@name "file"] | Dir of dir [@name "dir"]
and dir = { dir : string; files : file_or_dir list }

and worklist = { source : string; target : string; file_dirs : dir list }
[@@deriving yojson]

let rec parse_json json parent_dir target =
  let cur_dir = Filename.concat parent_dir json.dir in
  let target_dir = Filename.concat target json.dir in
  Core_unix.mkdir_p target_dir;
  List.iter json ~f:(function
    | { dir; files } -> List.fold
    | File file ->
        let source = Filename.concat cur_dir file in
        let target = Filename.concat target_dir file in
        FileUtil.cp [ source ] target
    | Dir worklist -> parse_json worklist cur_dir target_dir)

let () =
  let worklist = ref "" in
  let source = ref "" in
  let target = ref "" in
  Arg.parse
    [
      ( "-worklist",
        Arg.Set_string worklist,
        "JSON specifying which files to copy over." );
      ("-source", Arg.Set_string source, "Source directory to copy files from.");
      ("-target", Arg.Set_string target, "Target directory to copy files to.");
    ]
    (fun _ -> ())
    "./cpcpp.exe -worklist PATH_TO_JSON_FILE -target TARGET_DIR";
  let json =
    worklist_of_yojson @@ Safe.from_string @@ Core.In_channel.read_all !worklist
  in
  let source = json.source in
  let target = json.target in
  parse_json json.file_dirs source target
