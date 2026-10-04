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
