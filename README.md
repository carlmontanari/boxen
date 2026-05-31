<p align=center><a href="https://containerlab.srlinux.dev"><img src=https://gitlab.com/rdodin/pics/-/wikis/uploads/4a6e4555024a803e572eb2bf1ae83d74/boxen-logo-white-on-black_small.svg?sanitize=true/></a></p>

[![Go Report](https://img.shields.io/badge/go%20report-A%2B-blue?style=flat-square&color=00c9ff&labelColor=bec8d2)](https://goreportcard.com/report/github.com/carlmontanari/boxen)
[![License: MIT](https://img.shields.io/badge/License-MIT-blueviolet.svg?style=flat-square)](https://opensource.org/licenses/MIT)

---

boxen -- put your network operating systems in a box (or if you speak 🇩🇪, fight them! 🤣)!

boxen is a cli tool written in Go that allows you to package your network operating systems neatly 
into little... boxes (container images) so they are easily portable, and, most importantly, so you 
can use them with the wonderful [containerlab](https://github.com/srl-labs/containerlab).

## Attach to a VM console

When you want to add support for a new network operating system, you typically want to inspect the VM's boot process and prompts manually before coding them in the profile file. Use `boxen build --vm-console` to boot the VM and immediately attach your terminal to the VM serial console.

This mode still uses the normal build inputs, such as `--diskImage`/`--disk` and
`--profile`/`--prof`, but stops after the builder container has prepared and
started the VM.

```sh
boxen build --disk /path/to/disk.qcow2 --profile /path/to/profile.yaml --vm-console
```

Since the telnet client is used to attach to the VM console, to exit it, type `Ctrl+]` followed by `q`.