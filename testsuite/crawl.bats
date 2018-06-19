@test "invoking crawl with no options prints an error" {
  run $GOPATH/bin/crawl
  [ "$status" -eq 1 ]
  [[ "$output" =~ "Please provide a path to a file" ]]
}
