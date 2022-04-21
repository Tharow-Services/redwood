@rem #!/windows/system32/cmd.exe

go build -ldflags="-X 'main.Version=$(git describe --tags)' -X 'main.localServer=redwood.service'" 