# hyperoptic_tilgin_restart

Tool to restart Tilgin HG238x devices from the command line.
Only tested with Hyperoptic firmware.

## Running

### Building from source

```shell script
git clone https://github.com/cheahjs/hyperoptic_tilgin_restart.git
cd hyperoptic_tilgin_restart
go build -o hyperoptic_tilgin_restart github.com/cheahjs/hyperoptic_tilgin_restart/cmd/hyperoptic_tilgin_restart
ROUTER_PASSWORD=password ./hyperoptic_tilgin_restart -username=admin -host=http://192.168.1.1
```

### Docker

```shell script
docker run -e "ROUTER_PASSWORD=password" deathmax/hyperoptic_tilgin_restart -username=admin -host=http://192.168.1.1
```
