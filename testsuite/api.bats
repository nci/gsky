@test "invoking api with -h option prints usage" {
  run $GOPATH/bin/api -h
  [ "$status" -eq 2 ]
  [[ "${lines[0]}" =~ "Usage of " ]]
}
