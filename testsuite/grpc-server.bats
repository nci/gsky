@test "invoking grpc-server with invalid port number" {
    run $GOPATH/bin/grpc-server -p foo
    [ "$status" -eq 2 ]
    [[ "${lines[0]}" =~ invalid\ value\ \"foo\"\ for\ flag\ -p ]]
}

@test "invoking grpc-server with invalid number of workers" {
    run $GOPATH/bin/grpc-server -n foo
    [ "$status" -eq 2 ]
    [[ "${lines[0]}" =~ invalid\ value\ \"foo\"\ for\ flag\ -n ]]
}

@test "grpc-server runs on specified port (one thread)" {
    # needed so that grpc-server can run in the test environment
    ln -s $GOPATH/bin/gdal-process gsky-gdal-process
    run timeout 0.1s $GOPATH/bin/grpc-server -p 6001 -n 1
    [[ "${lines[0]}" =~ Process\ running\ with\ PID\ [[:digit:]]+ ]]
    [[ "${lines[1]}" =~ Listening\ on ]]
    rm gsky-gdal-process
}
