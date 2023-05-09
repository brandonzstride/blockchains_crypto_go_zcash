open Core

(* generate a complete .swigcxx file *)
let gen file_path to_include =
  let inner_incls =
    List.map to_include ~f:(fun incl -> "#include \"" ^ incl ^ "\"")
  in
  let outer_incls =
    List.map to_include ~f:(fun incl -> "%include \"" ^ incl ^ "\"")
  in
  let file_name =
    file_path |> Filename.basename |> String.split ~on:'.' |> List.hd_exn
  in
  (* concat as full list of file lines *)
  let swigcxx_ls =
    (("%module " ^ file_name) :: "%{" :: inner_incls) @ ("%}" :: outer_incls)
  in
  (* write lines to file *)
  Out_channel.write_all file_path ~data:(String.concat ~sep:"\n" swigcxx_ls)
