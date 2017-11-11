###  Lanproxy-go-client
go client for [lanproxy](https://github.com/ffay/lanproxy)

### QuickStart

Download precompiled [Releases](https://github.com/ffay/lanproxy/releases).

```
./client_darwin_amd64 -s SERVER_IP -p SERVER_PORT -k CLIENT_KEY
```
> eg: nohup ./client_darwin_amd64 -s lp.thingsglobal.org -p 4900 -k 01c1e176d6ee466c8db717a8 &

### Install from source

```
$go get -u github.com/ffay/lanproxy-go-client/src/main
```

All precompiled releases are genereated from `build-release.sh` script.