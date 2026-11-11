# EtherCAT ESI Files

This repository contains EtherCAT Slave Information (ESI) XML files from various vendors.

## Purpose

These files are used by the [Synnax](https://github.com/synnaxlabs/synnax) EtherCAT driver
to provide PDO (Process Data Object) definitions for devices that don't fully support
CoE SDO reads at runtime.

## Directory Structure

```
esi/
├── beckhoff/       # Beckhoff Automation devices
├── dewesoft/       # DEWESoft devices
├── supplements/    # Settings that Synnax shows for devices, by vendor
└── README.md
```

## Adding New Vendors

1. Create a directory for the vendor: `mkdir vendor_name`
2. Add ESI XML files to the directory
3. Generate the blob locally:
   ```bash
   cd parser
   go run . -catalog ../vendors.json -output-dir ../generated
   ```

## Releases

Each push to `main` publishes the registry blob under its own tag,
`registry-<commit>`. Synnax pins one tag and its SHA-256 in `vendor/esi.bzl`. A
published tag is never overwritten, because a changed file breaks the pinned
checksum of every Synnax checkout that uses it. The `latest` tag is frozen at
blob format 1 for older Synnax checkouts.

The blob format is versioned in the header. Format 2 keeps only the PDOs a device
assigns by default (the `Sm` attribute), keeps padding entries (index 0), and adds the
PDO name to each entry.

## Sources

- **Beckhoff**: https://download.beckhoff.com/download/configuration-files/io/ethercat/xml-device-description/
- **DEWESoft**: https://dewesoft.com/products/ethercat-accessories

## Statistics

| Vendor | Devices |
|--------|---------|
| Beckhoff Automation | 3,264 |
| DEWESoft | 103 |
| **Total** | **3,367** |

## License

ESI files are property of their respective vendors and are provided here for
interoperability purposes. Refer to individual vendor licenses for usage terms.

## Supplements

A vendor ESI often has no Dictionary, so Synnax cannot show the settings of the
device. A supplement is an ESI file that Synnax writes for such a device. It holds a
standard Dictionary that mirrors the object dictionary the device reports, and a
`Synnax` block under `VendorSpecific`. Other tools skip the block. It lists the
settings that Synnax shows, with three kinds of rules:

- `ShowWhen` shows a setting only when other settings have given values.
- `Require` rejects a channel whose settings break a rule, with a message.
- `OptionsFromCard` and `MaxFromCard` read option lists and limits from the device
  when the Driver scans it.

`Mode` names the object that sets the device up for Synnax alone (`Standalone`) or
for other software too. A `Value` with `SharedWith` is set up by that software, and
Synnax still reads its data. A `Value` with `OnlyReadBy` sends data that only that
software reads. `SecondPort` gives the revision bits of a device on its second EtherCAT
port; in dual mode, the first port belongs to the other software. `Save` keeps a
change through a power restart.

```xml
<Mode Index="#x8020" SubIndex="9" Standalone="0" SecondPort="#x00001000">
  <Value Value="0" AssignActivate="#x0700" Sync0Cycle="1000" Sync1Cycle="1000000000"/>
  <Value Value="1" SharedWith="DewesoftX"/>
  <Save Index="#x8020" SubIndex="2" Value="2" When="1"/>
</Mode>
```

A supplement applies to every revision of its product code, unless it lists revisions
in `Revisions`. A device matches when its revision, after `Mask`, is in the list. When
one product code names more than one module, each supplement of the code must list its
revisions with the same mask, and no two can list the same revision.

```xml
<Revisions Mask="#xFFFF0000">#x00050000 #x00060000</Revisions>
```

The parser writes all supplements to `registry_settings.inc`, a NUL-terminated JSON
document.
