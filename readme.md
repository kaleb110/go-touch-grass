go-touch-grass is a systemd background daemon for tracking daily time usage of your machine. reboot proof, persistant with history.

## install (if you have go installed)

```bash
go install github.com/kaleb110/go-touch-grass@latest
```

## setup (Linux, specifically debian)

### create systemd service in user space

```bash
mkdir -p ~/.config/systemd/user/
cp go-touch-grass.service ~/.config/systemd/user/go-touch-grass.service
```

### compile and move the binary 

```bash
chmod +x script.sh
./script.sh
```

### enable and start the service 

```bash
systemctl --user daemon-reload
systemctl --user enable go-touch-grass.service
systemctl --user start go-touch-grass.service
```

## reading logs
 
### view output using journal

```bash
journalctl --user-unit go-touch-grass.service # real time
journalctl --user-unit go-touch-grass.service -n 1 # latest
```

#### parse and show raw json

```bash
cat ~/.local/share/go-touch-grass/state.json
```



## running locally

```bash
go build -o go-touch-grass bin/main.go # build

./bin/go-touch-grass -tick=2 # run 
./bin/go-touch-grass -tick=2 -sim-tommorow # simulation for tommorow
```

## flag options

```-tick``` - time to update the state - in seconds
```sim-tommorow``` - login as tommorow, for simulation
