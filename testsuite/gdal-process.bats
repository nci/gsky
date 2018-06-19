@test "invoking gdal-process with -h prints usage" {
  run $GOPATH/bin/gdal-process -h
  [ "$status" -eq 2 ]
  [[ "${lines[0]}" =~ "Usage of " ]]
}

@test "invoking gdal-process with invalid flag -x prints an error" {
  run $GOPATH/bin/gdal-process -x
  [ "$status" -eq 2 ]
  [[ "${lines[0]}" = "flag provided but not defined: -x" ]]
}
