@test "invoking grpc-server with invalid port number" {
    run $GOPATH/bin/grpc-server -p foo
    [ "$status" -eq 2 ]
    [[ "${lines[0]}" =~ "invalid value \"foo\" for flag -p" ]]
}

@test "invoking grpc-server with invalid number of workers" {
    run $GOPATH/bin/grpc-server -n foo
    [ "$status" -eq 2 ]
    [[ "${lines[0]}" =~ "invalid value \"foo\" for flag -n" ]]
}
