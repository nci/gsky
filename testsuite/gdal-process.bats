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

@test "gdal-process listens on specified socket" {
    tmp=$(mktemp -u)
    run timeout 0.1s $GOPATH/bin/gdal-process -sock $tmp
    [ "$status" -eq 124 ]
    [[ "${lines[0]}" =~ "Listening on $tmp" ]]
    rm -f $tmp
}
