# Cisco CSR 1000v

The `cisco_csr1000v` profile packages an IOS XE QCOW2 disk. It uses 4096 MiB RAM, one CPU core, sixteen virtio data NICs, and transparent management at runtime.

## Disk

Use Cisco's serial console image, `csr1000v-universalk9.<version>-serial.qcow2`. The other images print their console to the VGA display, where the profile automation can not reach it. No companion files are needed.

## Package

```sh
boxen build --disk /path/to/images/csr1000v-universalk9.17.03.08a-serial.qcow2
```

The result is `boxen-cisco_csr1000v:17.03.08`; the version comes from the `universalk9.<version>` portion of the name. Packaging declines the setup dialog and autoinstall, waits for PnP discovery to stop, and saves a baseline: the `clab-mgmt` management VRF on GigabitEthernet1, SSH, NETCONF, and RESTCONF, and enabled data interfaces. The console opens in privileged exec mode without a login, which the runtime and save processes rely on.

## Run

Use Containerlab's [CSR 1000v kind](https://containerlab.dev/manual/kinds/vr-csr/) and the packaged image. At each start the profile sets the node hostname, creates the Containerlab user with privilege 15, and puts the container's management addresses and gateways on GigabitEthernet1 in the `clab-mgmt` VRF; with `CLAB_MGMT_DHCP=true` the interface uses DHCP. Data ports are GigabitEthernet2 and up, mapped to `eth1` and up.

A startup configuration is applied in configuration mode and saved with `write memory`. `boxen save` records `show running-config brief` without the self-signed trustpoint certificates, which IOS XE recreates, so the saved file can be applied the same way. It also leaves out `platform console`, `diagnostic bootup level`, and `license udi`: applying them rewrites the boot loader configuration, and stopping the container shortly afterwards can leave the boot loader without a configuration. Avoid these settings in startup configurations for the same reason. The data interfaces of the baseline are enabled, because the running configuration only lists `shutdown`.
