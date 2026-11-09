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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSupplement6xSTG(t *testing.T) {
	all, err := parseSupplements("../supplements")
	if err != nil {
		t.Fatal(err)
	}
	var stg *DeviceSettings
	for i := range all {
		if all[i].VendorID == 0xDEBE50F7 && all[i].ProductCode == 252 {
			stg = &all[i]
		}
	}
	if stg == nil {
		t.Fatal("no settings for the 6xSTG")
	}
	ch := stg.Channels
	if ch.Count != 6 || ch.IndexStep != 1 || ch.EntryIndex != 0x6030 ||
		ch.EntrySubIndex != 1 || ch.EntrySubIndexStep != 1 {
		t.Fatalf("channels = %+v", *ch)
	}
	measure := ch.Settings[0]
	want := Setting{
		Index:    0x8040,
		SubIndex: 1,
		Name:     "Measurement",
		DataType: "uint16",
		Options: []Option{
			{0, "Voltage"},
			{1, "Current"},
			{6, "Bridge"},
			{7, "Potentiometer"},
		},
	}
	if !reflect.DeepEqual(measure, want) {
		t.Fatalf("measurement = %+v", measure)
	}
	filter := ch.Settings[6]
	if filter.SubIndex != 4 || filter.Default != 1 || filter.Options != nil ||
		*filter.OptionsFrom != (OptionsFrom{0x8060, 1, 2, 1, 500, "kHz"}) {
		t.Fatalf("filter = %+v", filter)
	}
	exc := ch.Settings[10]
	if exc.SubIndex != 28 || exc.DataType != "float32" || exc.Unit != "V" ||
		*exc.MaxFrom != (MaxFrom{0x8070, 2, 10}) || len(exc.VisibleWhen) != 2 {
		t.Fatalf("excitation voltage = %+v", exc)
	}
	req := ch.Requires[0]
	if req.Then.SubIndex != 28 || *req.Then.Above != 0 ||
		!reflect.DeepEqual(req.When[0].Values, []float64{6, 7}) {
		t.Fatalf("require = %+v", req)
	}
	if len(stg.Device) != 2 || stg.Device[0].Default != 4 {
		t.Fatalf("device = %+v", stg.Device)
	}
}

func TestSupplementRejectsMissingSubIndex(t *testing.T) {
	dir := t.TempDir()
	xml := `<EtherCATInfo><Vendor><Id>#x2</Id></Vendor><Descriptions><Devices><Device>
<Type ProductCode="7">X</Type>
<Profile><Dictionary>
<DataTypes><DataType><Name>DT8000</Name>
<SubItem><SubIdx>1</SubIdx><Name>Mode</Name><Type>UINT</Type></SubItem>
</DataType></DataTypes>
<Objects><Object><Index>#x8000</Index><Type>DT8000</Type></Object></Objects>
</Dictionary></Profile>
<VendorSpecific><Synnax><DeviceSettings>
<Setting Index="#x8000" SubIndex="2"/>
</DeviceSettings></Synnax></VendorSpecific>
</Device></Devices></Descriptions></EtherCATInfo>`
	if err := os.WriteFile(filepath.Join(dir, "x.xml"), []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := parseSupplements(dir)
	if err == nil || !strings.Contains(err.Error(), "0x8000:02: no sub-index in DT8000") {
		t.Fatalf("err = %v", err)
	}
}

func TestSupplementRejectsConditionOnUnsetObject(t *testing.T) {
	dir := t.TempDir()
	xml := `<EtherCATInfo><Vendor><Id>#x2</Id></Vendor><Descriptions><Devices><Device>
<Type ProductCode="7">X</Type>
<Profile><Dictionary>
<DataTypes><DataType><Name>DT8000</Name>
<SubItem><SubIdx>1</SubIdx><Name>Mode</Name><Type>UINT</Type></SubItem>
<SubItem><SubIdx>2</SubIdx><Name>Gain</Name><Type>UINT</Type></SubItem>
</DataType></DataTypes>
<Objects><Object><Index>#x8000</Index><Type>DT8000</Type></Object></Objects>
</Dictionary></Profile>
<VendorSpecific><Synnax><DeviceSettings>
<Setting Index="#x8000" SubIndex="1"><ShowWhen Index="#x8000" SubIndex="2" Values="1"/></Setting>
</DeviceSettings></Synnax></VendorSpecific>
</Device></Devices></Descriptions></EtherCATInfo>`
	if err := os.WriteFile(filepath.Join(dir, "x.xml"), []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := parseSupplements(dir)
	want := "0x8000:02: a condition names an object that no setting sets"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %v", err)
	}
}
