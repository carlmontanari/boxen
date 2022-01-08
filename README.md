boxen
=====

[![Go Report](https://img.shields.io/badge/go%20report-A%2B-blue?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://goreportcard.com/report/github.com/carlmontanari/boxen)
[![License: MIT](https://img.shields.io/badge/License-MIT-blueviolet.svg?style=flat-square)](https://opensource.org/licenses/MIT)

---

boxen -- put your network operating systems in a box (or if you speak 🇩🇪, fight them! 🤣)! 

boxen is a cli tool written in Go that allows you to package your network operating systems neatly 
into little... boxes (container images) so they are easily portable, and, most importantly, so you 
can use them with the wonderful [containerlab](https://github.com/srl-labs/containerlab). boxen also
provides some basic functionality for running network operating systems on your local machine 
(without containers) -- even on Darwin systems (for platforms that work with HVF/HAX or no 
acceleration).

Please note that this is a work in progress... especially the documentation!


#### Key Features:

- __Easy__: It's easy to get going with boxen -- grab your network operating systems qcow2 disk and
  point boxen to it!
- __Native VMs__: Containers are cool, but sometimes (especially for Darwin systems) it is nice to
  launch network operating system VMs without containers/GUIs/etc. -- boxen has you covered!
- __Containerlab__: Love containerlab? Of course you do! We do too -- boxen was built to be used to
  package network operating systems up neatly for containerlab to orchestrate, it's a great match!


## Supported Platform Types

- Arista
  - vEOS (tested with 4.22.1F)
- Cisco
  - CSR1000v (tested with 16.12.03)
  - N9Kv (tested with 9.2.4)
  - XRv9K (tested with 6.5.3)
- Juniper
  - vSRX (tested with 17.3R2.10)
- Palo Alto
  - PA-VM (tested with 10.0.6)

Additional platforms can of course be added! 


## Installation

The simplest way to install boxen is to use the installation script:

```shell
# download and install the latest release
bash -c "$(curl -sL https://raw.githubusercontent.com/carlmontanari/boxen/main/get.sh)"

# download a specific version - 0.0.1
bash -c "$(curl -sL https://raw.githubusercontent.com/carlmontanari/boxen/main/get.sh)" -- -v 0.0.1

# with wget
bash -c "$(wget -qO - https://raw.githubusercontent.com/carlmontanari/boxen/main/get.sh)"
```


## Packaging For Containerlab

Packaging images for use with containerlab is easy! Snag the appropriate vmdk/qcow2 for your
platform of choice, and of course boxen. With those two things done, all you need to do is to run
boxen with the `package` command, and provide a disk to the `--disk` flag.

```bash
$ boxen package --disk ~/disks/nxosv.9.2.4.qcow2
      info   1640884754 package requested for disk 'nxosv.9.2.4.qcow2'
     debug   1640884754 temporary directory '/tmp/boxen2756986238' created successfully
     debug   1640884755 disks allocated for packaging
     debug   1640884755 packaging instance created
     debug   1640884756 bundling required packaging files complete
     debug   1640884756 pre packaging complete, begin docker-ization!
      info   1640884756 docker build output available at '/tmp/boxen2756986238/initial_build.log'
     debug   1640884995 base image building complete!
     debug   1640884995 package install starting
     debug   1640884995 begin instance install
      info   1640884995 install requested
      info   1640884995 qemu instance start requested
     debug   1640884995 launching instance with command: [-name cisco_n9kv -uuid 3c7fe9f9-61af-4c37-bf7b-338fd504f8ae -accel kvm -display none -machine pc -m 8192 -cpu max -smp cores=8,threads=1,sockets=1 -monitor tcp:0.0.0.0:4001,server,nowait -serial telnet:0.0.0.0:5001,server,nowait -drive if=none,file=disk.qcow2,format=qcow2,id=drive-sata-disk0 -device ahci,id=ahci0,bus=pci.0 -device ide-hd,drive=drive-sata-disk0,bus=ahci0.0,id=drive-sata-disk0,bootindex=1 -device pci-bridge,chassis_nr=1,id=pci.1 -device e1000,netdev=mgmt -netdev user,id=mgmt,net=10.0.0.0/24,tftp=/tftpboot,hostfwd=tcp::21022-10.0.0.15:22,hostfwd=tcp::21023-10.0.0.15:23,hostfwd=tcp::21443-10.0.0.15:443,hostfwd=tcp::21830-10.0.0.15:830,hostfwd=udp::31161-10.0.0.15:161 -device e1000,netdev=p001,bus=pci.1,addr=0x2,mac=52:54:00:54:6a:01 -netdev socket,id=p001,listen=:10001 -device e1000,netdev=p002,bus=pci.1,addr=0x3,mac=52:54:00:ee:56:02 -netdev socket,id=p002,listen=:10002 -device e1000,netdev=p003,bus=pci.1,addr=0x4,mac=52:54:00:4e:75:03 -netdev socket,id=p003,listen=:10003 -device e1000,netdev=p004,bus=pci.1,addr=0x5,mac=52:54:00:66:83:04 -netdev socket,id=p004,listen=:10004 -device e1000,netdev=p005,bus=pci.1,addr=0x6,mac=52:54:00:76:d0:05 -netdev socket,id=p005,listen=:10005 -device e1000,netdev=p006,bus=pci.1,addr=0x7,mac=52:54:00:66:25:06 -netdev socket,id=p006,listen=:10006 -device e1000,netdev=p007,bus=pci.1,addr=0x8,mac=52:54:00:6d:8b:07 -netdev socket,id=p007,listen=:10007 -device e1000,netdev=p008,bus=pci.1,addr=0x9,mac=52:54:00:34:99:08 -netdev socket,id=p008,listen=:10008 -bios ./OVMF.fd -boot c]
     debug   1640884995 stdout logger provided, setting execute argument
     debug   1640884995 stderr logger provided, setting execute argument
      info   1640885005 qemu instance start complete
     debug   1640885005 instance started, waiting for start ready state
      info   1640885015 install logs available at '/tmp/boxen2756986238/install_build.log', or by inspect container 'b54b335336c667df60dd04aaaec88b5adf9d455aa46a2f17638239228dbc8d93' logs
     debug   1640885199 start ready state acquired, handling initial config dialog
     debug   1640885211 initial config dialog addressed, logging in
     debug   1640885217 log in complete
     debug   1640885217 install config lines provided, executing scrapligo on open
     debug   1640885225 initial installation complete
      info   1640885225 save config requested
      info   1640885230 install complete, stopping instance
      info   1640885230 qemu instance stop requested
      info   1640885230 qemu instance stop complete
     debug   1640885240 instance installation complete!
      info   1640885280 final image build logs available at '/tmp/boxen2756986238/final_build.log'
     debug   1640885280 packaging complete!
	✅ finished successfully in 527 seconds
```

This "package" command will copy the disk image to a temporary directory and write out a few
Dockerfiles that will be used for the packaging process. The first dockerfile is the "build" image;
this image gets the disk and any necessary files (OVMF.fd, config.iso, etc.) copied into it. Once
the build image is created, a container is spawned from that image -- this "install container" is
where the initial device prompts (POAP, ZTP, etc.) is dealt with, and a base configuration is
deployed. Once the initial configuration is sorted, the configuration is saved, and the container is
stopped.

With the initial "installation" done, the disk image is copied out of the stopped container. Finally,
a final slimmer container is built, which includes copying in the freshly installed disk.

At that point, the final image will be tagged "boxen_vendor_platform" with a version of whatever the
provided disk version was.


## Packaging for local VM Use

The process for setting up local VMs is somewhat similar to the packaging setup. First things first
is that boxen will need a nice little directory to store its config file as well as any source disks
and of course instance disks. The `init` command initializes this boxen directory, which by default
is `~/boxen`, but you can provide whatever path you like to the `--directory` flag.

With boxen "initialized" you are set to "install" a disk as a local source disk. This is done with
the appropriately named `install` flag.

`boxen install --disk somedisk.qcow2`

Note that by default boxen will look for its config file at `~/boxen/boxen.yaml` -- if you
initialized boxen with a different path, make sure you pass the config file path with the `--config`
flag!

Just like the packaging process, boxen will automagically determine the vendor, platform and disk
version from the provided image. The "installation" process is pretty similar to the packaging
process, just without containers of course. The disk is copied to a temp directory and is launched
via qemu. The initial configuration process is sorted out, and the configuration is saved. Once
complete, the new "source" disk is copied to the "source" sub-directory in the boxen directory.

At this point the "source" disk is ready, but we have no instances to launch!

The boxen `provision` command allows for provisioning one or many of the same type of instance. If
you just installed an Arista vEOS source disk you could provision two vEOS instances like:

`boxen provision --vendor arista --platform veos --instances eos1,eos2`

The "provisioning" process simply installs these instances into the boxen config file and allocates
instance IDs, management interface nat ports, and data plane interface listen ports. You can view
the config file to see all of these settings.

Once provisioned you can start or stop instances easily:

`boxen start instance --instances eos1,eos2`

`boxen stop instance --instances eos2`


## Other Info

### Sparsify Disks

Some platforms will support disk "sparsification" -- to enable this, run boxen with the 
`BOXEN_SPARSIFY_DISK` env var set to anything > 0. 


### Dev Mode

During installation/dev/testing and such it is very handy to *not* delete boxen's temp build 
directory, you can enable this behavior by setting the `BOXEN_DEV_MODE` env var to anything > 0.


### Log Level

`BOXEN_LOG_LEVEL` env var can be set to "info", "debug" or "critical" (default info) to control log
verbosity. Note that this environment variable is copied into any "packaged" containers -- so if you
want to have debug level logging for your containerlab images, you should package the instance with
this flag set!


### Timeout Multiplier

`BOXEN_TIMEOUT_MULTIPLIER` does what it says on the tin -- mostly this just modifies how long to
wait for console availability and for "read until" operations. This setting is also copied into any
packaged containers (see log level note).


### Quirks

- vSRX will accept unencrypted passwords and do poor md5 encryption on them such that they can be
  sent to the device without needing interaction.
- PanOS should very, very much be packaged with sparsify set! Without this the image is huge (>8gb),
  but with sparsify enabled it is a much more manageable (but still large) ~3gb. 
- Boxen totally does not care about you! Well... it *kind of* doesn't care about you. Boxen
  generally requires elevated permissions -- we run containers with privileged flag, this allows the
  container to access KVM acceleration, we also launch all "local" qemu instances with sudo which
  allows for taps and bridges and acceleration and all that stuff. If you are *not* a passwordless
  super user you have two choices about how to let boxen run things as root. 1) you can simply run
  the entire boxen binary as root ("sudo boxen blah"), or 2) you can let boxen prompt you and it
  will only use "sudo" when it needs to. The part where boxen does not care about you is that if you
  opt for option two, your sudo password will be leaked into your bash/zsh history as we are lazy
  (and don't care about you!) and basically run "sh -c 'echo 'yourpassword' | sudo -S somecommand".
  TL:DR -- if you care about your password being in your bash history either be a passwordless sudo
  user, or run the boxen binary as root!


### VM Acceleration Notes

Typically, when launching Qemu virtual machines, KVM acceleration is enabled -- especially for
network operating system VMs. This is all well and good, however KVM is *not* available on Darwin
systems (obviously). But! HVF (hypervisor framework) and [HAX](https://github.com/intel/haxm) *are*
available on Darwin systems, and thankfully some NOS do seem to boot nicely with either HVF or HAX.

Boxen auto-selects the "best" acceleration option available for the environment the VM is being
launched. For packaged instances this will always be KVM, and as such you must always run the
container on a Linux system with KVM available. For local VMs, KVM is always the most preferred
option, however each platform contains a simple slice of supported accelerations (in order of
priority), and will be booted with the first available acceleration.

For Darwin users, it is highly recommend to install HAXM (see link above).



## TODO

- Output mgmt interface details when starting local VM instances