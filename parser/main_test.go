// Copyright 2026 Synnax Labs, Inc.
//
// Use of this software is governed by the Business Source License included in the file
// licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with the Business Source
// License, use of this software will be governed by the Apache License, Version 2.0,
// included in the file licenses/APL.txt.

package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

const fixtureESI = `<?xml version="1.0" encoding="ISO-8859-1"?>
<EtherCATInfo>
  <Vendor><Id>#x00000002</Id><Name>Beckhoff Automation GmbH</Name></Vendor>
  <Descriptions><Devices>
    <Device>
      <Type ProductCode="#x0c1e3052" RevisionNo="#x00140000">EL3102</Type>
      <Name LcId="1031">EL3102 2K. Ana. Eingang</Name>
      <Name LcId="1033">EL3102 2Ch. Ana. Input</Name>
      <TxPdo Sm="3">
        <Index>#x1a00</Index>
        <Name LcId="1031">Kanal 1</Name>
        <Name LcId="1033">Channel 1</Name>
        <Entry><Index>#x6000</Index><SubIndex>1</SubIndex><BitLen>1</BitLen>
          <Name>Underrange</Name><DataType>BOOL</DataType></Entry>
        <Entry><Index>#x0</Index><BitLen>7</BitLen></Entry>
        <Entry><Index>#x6000</Index><SubIndex>17</SubIndex><BitLen>16</BitLen>
          <Name>Value</Name><DataType>INT</DataType></Entry>
      </TxPdo>
      <TxPdo Sm="3">
        <Index>#x1a01</Index>
        <Name>Channel 2</Name>
        <Entry><Index>#x6010</Index><SubIndex>17</SubIndex><BitLen>16</BitLen>
          <Name>Value</Name><DataType>INT</DataType></Entry>
      </TxPdo>
      <TxPdo>
        <Index>#x1a03</Index>
        <Name>AI Compact Channel 1</Name>
        <Exclude>#x1a00</Exclude>
        <Entry><Index>#x6000</Index><SubIndex>17</SubIndex><BitLen>16</BitLen>
          <Name>Value</Name><DataType>INT</DataType></Entry>
      </TxPdo>
      <RxPdo Sm="2">
        <Index>#x1600</Index>
        <Name>Channel 1</Name>
        <Entry><Index>#x7000</Index><SubIndex>1</SubIndex><BitLen>1</BitLen>
          <Name>Output</Name><DataType>BOOL</DataType></Entry>
      </RxPdo>
    </Device>
  </Devices></Descriptions>
</EtherCATInfo>`

func parseFixture(t *testing.T) ParsedDevice {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.xml")
	if err := os.WriteFile(path, []byte(fixtureESI), 0o644); err != nil {
		t.Fatal(err)
	}
	devices, err := parseESIFile(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	return devices[0]
}

func TestParseESIFile(t *testing.T) {
	t.Run("should keep only PDOs assigned to a sync manager by default", func(t *testing.T) {
		dev := parseFixture(t)
		for _, e := range dev.InputPDOs {
			if e.PDOIndex == 0x1A03 {
				t.Fatalf("unassigned PDO 0x1A03 was kept: %+v", e)
			}
		}
		if len(dev.InputPDOs) != 4 {
			t.Fatalf("expected 4 input entries, got %d: %+v", len(dev.InputPDOs), dev.InputPDOs)
		}
	})

	t.Run("should store the PDO name with each entry, preferring English", func(t *testing.T) {
		dev := parseFixture(t)
		want := []string{"Channel 1", "Channel 1", "Channel 1", "Channel 2"}
		for i, e := range dev.InputPDOs {
			if e.PDOName != want[i] {
				t.Fatalf("entry %d: expected PDO name %q, got %q", i, want[i], e.PDOName)
			}
		}
		if dev.Name != "EL3102 2Ch. Ana. Input" {
			t.Fatalf("expected English device name, got %q", dev.Name)
		}
	})

	t.Run("should keep padding entries in mapping order", func(t *testing.T) {
		dev := parseFixture(t)
		gap := dev.InputPDOs[1]
		if gap.Index != 0 || gap.SubIndex != 0 || gap.BitLength != 7 || gap.Name != "" {
			t.Fatalf("expected a 7-bit padding entry, got %+v", gap)
		}
		if dev.InputPDOs[2].Index != 0x6000 || dev.InputPDOs[2].SubIndex != 17 {
			t.Fatalf("expected Value after the gap, got %+v", dev.InputPDOs[2])
		}
	})

	t.Run("should parse output PDOs the same way", func(t *testing.T) {
		dev := parseFixture(t)
		if len(dev.OutputPDOs) != 1 || dev.OutputPDOs[0].PDOName != "Channel 1" {
			t.Fatalf("unexpected outputs: %+v", dev.OutputPDOs)
		}
	})
}

func TestBuildBlob(t *testing.T) {
	t.Run("should encode each entry with its PDO name in a 16 byte record", func(t *testing.T) {
		dev := parseFixture(t)
		blob := buildBlob([]ParsedDevice{dev})
		le := binary.LittleEndian
		if v := le.Uint32(blob[4:8]); v != BinaryVersion {
			t.Fatalf("expected version %d, got %d", BinaryVersion, v)
		}
		vendorCount := le.Uint32(blob[8:12])
		indexCount := le.Uint32(blob[12:16])
		deviceCount := le.Uint32(blob[16:20])
		stringsAt := le.Uint32(blob[24:28])
		pdosAt := 32 + vendorCount*8 + indexCount*16 + deviceCount*16
		str := func(off uint32) string {
			start := stringsAt + off
			end := start
			for blob[end] != 0 {
				end++
			}
			return string(blob[start:end])
		}
		const recordSize = 16
		value := blob[pdosAt+2*recordSize : pdosAt+3*recordSize]
		if le.Uint16(value[2:4]) != 0x6000 || value[4] != 17 {
			t.Fatalf("unexpected third record: % x", value)
		}
		if got := str(le.Uint32(value[8:12])); got != "Value" {
			t.Fatalf("expected entry name Value, got %q", got)
		}
		if got := str(le.Uint32(value[12:16])); got != "Channel 1" {
			t.Fatalf("expected PDO name Channel 1, got %q", got)
		}
	})
}
