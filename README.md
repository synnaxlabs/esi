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

A supplement applies to every revision of its product code. The parser writes all
supplements to `registry_settings.inc`, a NUL-terminated JSON document.
