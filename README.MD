# TMI (Thermal Management Interface)

***tmi*** is a simple program to remotely (or locally) control fan/pump speeds using IPMI (Intelligent Platform Management Interface) and/or a Corsair Commander Pro, based on the readings of temperature sensors (CPU, GPU or any sensor you can grab via command line).  
Simply install it and tweaks config files as needed, they will be hot-reloaded automatically after saving changes.

>#### WARNING
>This project is born because I wanted to handle the thermal/acoustic behaviour of my personal workstation so it is not intended to be used in production environments.  
This program is of public domain and provided as-is, without any warranty whatsoever.  
It has only been tested on a SuperMicro X11SPA-TF motherboard running Ubuntu 18.04 and 19.10.  
`tmi` sends raw IPMI commands to your IPMI device and to the Commander Pro and may not work on your platform without some modifications.  
**USE AT YOUR OWN RISK**.

## Features
- control ipmi fan zones duty-cycle.
- control ipmi fans thresholds.
- get ipmi temperature from sensors.
- control Commander Pro fans duty-cycle.
- control Commander Pro leds (basic control).
- get Commander Pro temp from sensors.
- get temp from any custom CLI command.


## Requirements
Precompiled executable are inside artifacts dir, config files must reside in the working dir if you use these ...or you can build it for yourself if you have `go` installed using the included makefile for convenience.  


- `ipmitool` to use IPMI. 
- `libusb-1.0` to communicate with the Corsair Commander Pro. 

## Quick start

#### Ubuntu / Debian:
Simply use the makefile and adjust the three config files according to your needs:
```sh
sudo make install_linux path=/opt/tmi
```
This will:
- Build the executable for the current platform to the specified path (eg.:/opt/tmi).
- Copy the config files to the same path (will not be overwritten if exist).
- Install `ipmitool` if not already installed.
- Setup a systemd service (tmi), enable and start it. 

![service](tmi.png)

At this point you just have to play with the configs ([`tmi.yaml`](artifacts/tmi.yaml), [`ipmi.yaml`](artifacts/ipmi.yaml) and [`commanderpro.yaml`](artifacts/commanderpro.yaml)) which has been copied in '/opt/tmi'.  
They will be reloaded automatically after saving changes, just wait for the next check cycle (check_interval is defined in the tmi.yaml config itself).  
You can monitor `tmi` status at any time with:
````bash
journalctl -f -u tmi
````

#### Other platforms:
1. Ensure target machine has `ipmitool` and `libusb` installed.  
    On OSX use brew:
    ```bash
    brew install ipmitool
    brew install libusb
    ```
2. If you have `go` installed build the executable and copy the configs to the desired path.
    ```sh
    sudo make build path=/opt/tmi
    ```
   ...otherwise manually copy the right precompiled executable and both config files in the same destination path.
3. Run the executable as a daemon.
4. Monitor its stdout.
5. Stress your machine to see if it works!

## Configuration & Usage

Three config files are required for `tmi` to run.  
Anytime you save some changes the configs are hot-reloaded (separately).  
The configuration options are documented in the sample files.

[`tmi.yaml`](artifacts/tmi.yaml):  

```yaml
# disable the modules that you don't use.
active_modules:
  ipmi: true
  commanderpro: true

# Number of seconds to sleep between checks.
# Check configuration changes and sensors data every x seconds.
check_interval: 6

# Create a targets map to be used as reference inside the controllers configuration below.
# <arbitrary_name>: <fan_controller>.<fan_controller_channel>
targets_map:
  pump: ipmi.0
  side: commanderpro.0
  top: commanderpro.1
  front: commanderpro.2
  rear: commanderpro.3

controllers:

  # name is arbitrary, will be printed in logs.
  - name: CPU
    # The minimum change in °C from the last update to actually cause another fan speed change.
    min_temp_change: 4
    # IPMI sensor entityID to look for.
    # Get the ipmi sensor entityID with: `sudo ipmitool sdr elist full` at the fourth column in result.
    # ... or with: `sudo ipmitool sensor get <sensor_id>` (eg.: sudo ipmitool sensor get 'CPU Temp')
    temp:
      # commanderpro, ipmi, cli
      method: ipmi
      # commanderpro: sensor_channel (uint8 as string), ipmi: entityID, cli: custom_command
      arg: 3.1
    # Control multiple target zones with the same sensor...
    targets:
      pump:
        0:  30
        36: 50
        54: 72
        60: 100
      side:
        0: 0
        35: 30
        45: 50
        55: 75
        65: 100
      top:
        0: 18
        35: 30
        45: 50
        55: 75
        65: 100

  - name: GPU
    min_temp_change: 2
    # CLI command to extract NVIDIA GPU temp.
    # Should return a string representing a valid float number,
    # leading and trailing spaces will be automatically removed.
    temp:
      method: cli
      arg: nvidia-smi --query-gpu=temperature.gpu --format=csv,noheader
    targets:
      pump:
        0: 30
        45: 50
        55: 75
        65: 100
      side:
        0: 0
        35: 30
        45: 50
        55: 75
        65: 100
      top:
        0: 18
        35: 30
        45: 50
        55: 75
        65: 100

  - name: PCH
    min_temp_change: 3
    # temp_corsair_channel grab the temperature from a channel of the corsair commander pro
    temp:
      method: ipmi
      arg: 7.1
    targets:
      front:
        0: 34
        56: 70
        63: 100
      rear:
        0: 18
        53: 30
        56: 50
        63: 100
```
