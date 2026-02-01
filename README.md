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
3. Run the parser from the Synnax repo:
   ```bash
   cd synnax/driver/ethercat/esi
   go run main.go /path/to/esi known_devices.h
   ```

## Sources

- **Beckhoff**: https://download.beckhoff.com/download/configuration-files/io/ethercat/xml-device-description/
- **DEWESoft**: https://dewesoft.com/products/ethercat-accessories

## Statistics

| Vendor | Devices |
|--------|---------|
| Beckhoff Automation | 3,271 |
| DEWESoft | 103 |
| **Total** | **3,374** |

## License

ESI files are property of their respective vendors and are provided here for
interoperability purposes. Refer to individual vendor licenses for usage terms.
