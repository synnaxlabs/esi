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
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html/charset"
)

// A supplement is an ESI file that Synnax writes for a device whose vendor ESI lacks a
// Dictionary. It holds a standard Dictionary and a Synnax block under VendorSpecific
// that says which settings Synnax shows, when, and with which options.

type supplementInfo struct {
	Vendor       Vendor `xml:"Vendor"`
	Descriptions struct {
		Devices []supplementDevice `xml:"Devices>Device"`
	} `xml:"Descriptions"`
}

type supplementDevice struct {
	Type       DeviceType  `xml:"Type"`
	Dictionary dictionary  `xml:"Profile>Dictionary"`
	Synnax     *synnaxInfo `xml:"VendorSpecific>Synnax"`
}

type dictionary struct {
	DataTypes []dictDataType `xml:"DataTypes>DataType"`
	Objects   []dictObject   `xml:"Objects>Object"`
}

type dictDataType struct {
	Name     string        `xml:"Name"`
	BaseType string        `xml:"BaseType"`
	Enums    []dictEnum    `xml:"EnumInfo"`
	SubItems []dictSubItem `xml:"SubItem"`
}

type dictEnum struct {
	Text string `xml:"Text"`
	Enum string `xml:"Enum"`
}

type dictSubItem struct {
	SubIdx string `xml:"SubIdx"`
	Name   string `xml:"Name"`
	Type   string `xml:"Type"`
}

type dictObject struct {
	Index string `xml:"Index"`
	Type  string `xml:"Type"`
	Info  []struct {
		Name        string `xml:"Name"`
		DefaultData string `xml:"Info>DefaultData"`
	} `xml:"Info>SubItem"`
}

type synnaxInfo struct {
	Channels       *synnaxChannels `xml:"Channels"`
	DeviceSettings []synnaxSetting `xml:"DeviceSettings>Setting"`
}

type synnaxChannels struct {
	Count     int             `xml:"Count,attr"`
	IndexStep string          `xml:"IndexStep,attr"`
	Entries   []synnaxEntries `xml:"Entries"`
	Settings  []synnaxSetting `xml:"Setting"`
	Requires  []synnaxRequire `xml:"Require"`
}

type synnaxEntries struct {
	Index        string `xml:"Index,attr"`
	IndexStep    string `xml:"IndexStep,attr"`
	SubIndex     string `xml:"SubIndex,attr"`
	SubIndexStep string `xml:"SubIndexStep,attr"`
	Count        string `xml:"Count,attr"`
}

type synnaxSetting struct {
	Index           string            `xml:"Index,attr"`
	SubIndex        string            `xml:"SubIndex,attr"`
	Name            string            `xml:"Name,attr"`
	Unit            string            `xml:"Unit,attr"`
	Min             string            `xml:"Min,attr"`
	ShowWhen        []synnaxCondition `xml:"ShowWhen"`
	Options         []synnaxOption    `xml:"Option"`
	OptionsFromCard *struct {
		Index         string `xml:"Index,attr"`
		CountSubIndex string `xml:"CountSubIndex,attr"`
		FirstSubIndex string `xml:"FirstSubIndex,attr"`
		FirstValue    string `xml:"FirstValue,attr"`
		Divisor       string `xml:"Divisor,attr"`
		Unit          string `xml:"Unit,attr"`
	} `xml:"OptionsFromCard"`
	MaxFromCard *struct {
		Index    string `xml:"Index,attr"`
		SubIndex string `xml:"SubIndex,attr"`
		Divisor  string `xml:"Divisor,attr"`
	} `xml:"MaxFromCard"`
}

type synnaxCondition struct {
	Index    string `xml:"Index,attr"`
	SubIndex string `xml:"SubIndex,attr"`
	Values   string `xml:"Values,attr"`
	Above    string `xml:"Above,attr"`
}

type synnaxOption struct {
	Value string `xml:"Value,attr"`
	Name  string `xml:"Name,attr"`
}

type synnaxRequire struct {
	synnaxCondition
	Message string            `xml:"Message,attr"`
	When    []synnaxCondition `xml:"When"`
}

// DeviceSettings is the settings description of one product, as the Driver reads it.
// The Driver reads each OptionsFrom and MaxFrom from the slave at scan time.
type DeviceSettings struct {
	VendorID    uint32           `json:"vendor_id"`
	ProductCode uint32           `json:"product_code"`
	Device      []Setting        `json:"device"`
	Channels    *ChannelSettings `json:"channels,omitempty"`
}

// ChannelSettings describes the settings of Count channels. Channel n reads the entries
// of each item in Entries, and adds n * IndexStep to every index in Settings and
// Requires.
type ChannelSettings struct {
	Count     int       `json:"count"`
	IndexStep uint16    `json:"index_step"`
	Entries   []Entries `json:"entries"`
	Settings  []Setting `json:"settings"`
	Requires  []Require `json:"requires"`
}

// Entries are the PDO entries that channel n reads in one PDO layout: Count entries
// from (Index + n * IndexStep):(SubIndex + n * SubIndexStep), one sub-index apart.
type Entries struct {
	Index        uint16 `json:"index"`
	IndexStep    uint16 `json:"index_step"`
	SubIndex     uint8  `json:"sub_index"`
	SubIndexStep uint8  `json:"sub_index_step"`
	Count        uint8  `json:"count"`
}

// Setting is one CoE object that Synnax shows as a field.
type Setting struct {
	Index       uint16       `json:"index"`
	SubIndex    uint8        `json:"sub_index"`
	Name        string       `json:"name"`
	DataType    string       `json:"data_type"`
	Default     float64      `json:"default"`
	Unit        string       `json:"unit,omitempty"`
	Min         *float64     `json:"min,omitempty"`
	Options     []Option     `json:"options,omitempty"`
	OptionsFrom *OptionsFrom `json:"options_from,omitempty"`
	MaxFrom     *MaxFrom     `json:"max_from,omitempty"`
	VisibleWhen []Condition  `json:"visible_when,omitempty"`
}

// Option is one value of a setting with a fixed set of values.
type Option struct {
	Value float64 `json:"value"`
	Name  string  `json:"name"`
}

// OptionsFrom reads the options of a setting from a table on the slave: a count at
// CountSubIndex, then that many values from FirstSubIndex. Option i has the value
// FirstValue + i and the name of its table value divided by Divisor, then Unit.
type OptionsFrom struct {
	Index         uint16  `json:"index"`
	CountSubIndex uint8   `json:"count_sub_index"`
	FirstSubIndex uint8   `json:"first_sub_index"`
	FirstValue    float64 `json:"first_value"`
	Divisor       float64 `json:"divisor"`
	Unit          string  `json:"unit"`
}

// MaxFrom reads the largest value of a setting from the slave, divided by Divisor.
type MaxFrom struct {
	Index    uint16  `json:"index"`
	SubIndex uint8   `json:"sub_index"`
	Divisor  float64 `json:"divisor"`
}

// Condition holds when the object at Index:SubIndex is one of Values, or above Above.
type Condition struct {
	Index    uint16    `json:"index"`
	SubIndex uint8     `json:"sub_index"`
	Values   []float64 `json:"values,omitempty"`
	Above    *float64  `json:"above,omitempty"`
}

// Require rejects a channel whose settings meet every condition in When but not Then.
type Require struct {
	When    []Condition `json:"when"`
	Then    Condition   `json:"then"`
	Message string      `json:"message"`
}

// settingDataTypes maps the ESI base types a setting can have to Synnax data types.
var settingDataTypes = map[string]string{
	"SINT":  "int8",
	"USINT": "uint8",
	"INT":   "int16",
	"UINT":  "uint16",
	"DINT":  "int32",
	"UDINT": "uint32",
	"REAL":  "float32",
	"LREAL": "float64",
}

func parseSupplementFile(path string) ([]DeviceSettings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = func(label string, input io.Reader) (io.Reader, error) {
		return charset.NewReaderLabel(label, input)
	}
	var info supplementInfo
	if err := decoder.Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	vendorID, err := parseHexOrDecimal(info.Vendor.ID)
	if err != nil {
		return nil, fmt.Errorf("%s: vendor ID: %w", path, err)
	}
	var result []DeviceSettings
	for _, dev := range info.Descriptions.Devices {
		if dev.Synnax == nil {
			continue
		}
		productCode, err := parseHexOrDecimal(dev.Type.ProductCode)
		if err != nil {
			return nil, fmt.Errorf("%s: product code: %w", path, err)
		}
		settings, err := resolveSupplement(dev)
		if err != nil {
			return nil, fmt.Errorf("%s: product %d: %w", path, productCode, err)
		}
		settings.VendorID = uint32(vendorID)
		settings.ProductCode = uint32(productCode)
		result = append(result, settings)
	}
	return result, nil
}

func resolveSupplement(dev supplementDevice) (DeviceSettings, error) {
	dict := dev.Dictionary
	out := DeviceSettings{Device: []Setting{}}
	for _, s := range dev.Synnax.DeviceSettings {
		setting, err := resolveSetting(dict, s)
		if err != nil {
			return out, err
		}
		out.Device = append(out.Device, setting)
	}
	if err := checkConditions(out.Device, nil); err != nil {
		return out, err
	}
	ch := dev.Synnax.Channels
	if ch == nil {
		return out, nil
	}
	settings := &ChannelSettings{
		Count:    ch.Count,
		Entries:  []Entries{},
		Settings: []Setting{},
		Requires: []Require{},
	}
	var err error
	if settings.IndexStep, err = parseUint16(ch.IndexStep); err != nil {
		return out, fmt.Errorf("IndexStep: %w", err)
	}
	if len(ch.Entries) == 0 {
		return out, fmt.Errorf("Channels: no Entries")
	}
	for _, e := range ch.Entries {
		entries, err := parseEntries(e)
		if err != nil {
			return out, fmt.Errorf("Entries: %w", err)
		}
		settings.Entries = append(settings.Entries, entries)
	}
	for _, s := range ch.Settings {
		setting, err := resolveSetting(dict, s)
		if err != nil {
			return out, err
		}
		settings.Settings = append(settings.Settings, setting)
	}
	for _, r := range ch.Requires {
		req := Require{Message: r.Message}
		if req.Then, err = parseCondition(r.synnaxCondition); err != nil {
			return out, fmt.Errorf("Require: %w", err)
		}
		for _, w := range r.When {
			cond, err := parseCondition(w)
			if err != nil {
				return out, fmt.Errorf("Require When: %w", err)
			}
			req.When = append(req.When, cond)
		}
		settings.Requires = append(settings.Requires, req)
	}
	if err := checkConditions(settings.Settings, settings.Requires); err != nil {
		return out, err
	}
	out.Channels = settings
	return out, nil
}

// parseEntries reads e. A missing step is 0, and a missing Count is 1.
func parseEntries(e synnaxEntries) (Entries, error) {
	out := Entries{Count: 1}
	var err error
	if out.Index, err = parseUint16(e.Index); err != nil {
		return out, fmt.Errorf("Index: %w", err)
	}
	if out.SubIndex, err = parseUint8(e.SubIndex); err != nil {
		return out, fmt.Errorf("SubIndex: %w", err)
	}
	if e.IndexStep != "" {
		if out.IndexStep, err = parseUint16(e.IndexStep); err != nil {
			return out, fmt.Errorf("IndexStep: %w", err)
		}
	}
	if e.SubIndexStep != "" {
		if out.SubIndexStep, err = parseUint8(e.SubIndexStep); err != nil {
			return out, fmt.Errorf("SubIndexStep: %w", err)
		}
	}
	if e.Count != "" {
		if out.Count, err = parseUint8(e.Count); err != nil {
			return out, fmt.Errorf("Count: %w", err)
		}
	}
	if out.Count == 0 {
		return out, fmt.Errorf("Count: must be at least 1")
	}
	return out, nil
}

// checkConditions returns an error when a condition names an object that none of the
// settings set.
func checkConditions(settings []Setting, requires []Require) error {
	set := make(map[[2]int]bool, len(settings))
	var conds []Condition
	for _, s := range settings {
		set[[2]int{int(s.Index), int(s.SubIndex)}] = true
		conds = append(conds, s.VisibleWhen...)
	}
	for _, r := range requires {
		conds = append(append(conds, r.When...), r.Then)
	}
	for _, c := range conds {
		if !set[[2]int{int(c.Index), int(c.SubIndex)}] {
			return fmt.Errorf(
				"0x%04X:%02X: a condition names an object that no setting sets",
				c.Index, c.SubIndex,
			)
		}
	}
	return nil
}

// resolveSetting fills a setting from the Dictionary entry it names. An option list,
// name, or unit in the Synnax block replaces what the Dictionary gives.
func resolveSetting(dict dictionary, s synnaxSetting) (Setting, error) {
	index, err := parseUint16(s.Index)
	if err != nil {
		return Setting{}, fmt.Errorf("Setting Index: %w", err)
	}
	subIndex, err := parseUint8(s.SubIndex)
	if err != nil {
		return Setting{}, fmt.Errorf("Setting SubIndex: %w", err)
	}
	addr := fmt.Sprintf("0x%04X:%02X", index, subIndex)
	obj, ok := findObject(dict, index)
	if !ok {
		return Setting{}, fmt.Errorf("%s: no object in the Dictionary", addr)
	}
	objType, ok := findDataType(dict, obj.Type)
	if !ok {
		return Setting{}, fmt.Errorf("%s: no data type %s", addr, obj.Type)
	}
	pos := -1
	for i, sub := range objType.SubItems {
		if n, err := parseUint8(sub.SubIdx); err == nil && n == subIndex {
			pos = i
			break
		}
	}
	if pos < 0 {
		return Setting{}, fmt.Errorf("%s: no sub-index in %s", addr, obj.Type)
	}
	sub := objType.SubItems[pos]
	setting := Setting{Index: index, SubIndex: subIndex, Name: sub.Name, Unit: s.Unit}
	baseType := sub.Type
	if enum, ok := findDataType(dict, sub.Type); ok {
		baseType = enum.BaseType
		for _, e := range enum.Enums {
			v, err := strconv.ParseFloat(strings.TrimSpace(e.Enum), 64)
			if err != nil {
				return Setting{}, fmt.Errorf("%s: enum %q: %w", addr, e.Enum, err)
			}
			setting.Options = append(setting.Options, Option{Value: v, Name: e.Text})
		}
	}
	dataType, ok := settingDataTypes[strings.ToUpper(baseType)]
	if !ok {
		return Setting{}, fmt.Errorf("%s: unsupported type %s", addr, baseType)
	}
	setting.DataType = dataType
	if pos < len(obj.Info) && obj.Info[pos].DefaultData != "" {
		if obj.Info[pos].Name != sub.Name {
			return Setting{}, fmt.Errorf(
				"%s: the object lists %q where its type lists %q",
				addr,
				obj.Info[pos].Name,
				sub.Name,
			)
		}
		if setting.Default, err = decodeDefault(obj.Info[pos].DefaultData, dataType); err != nil {
			return Setting{}, fmt.Errorf("%s: DefaultData: %w", addr, err)
		}
	}
	if s.Name != "" {
		setting.Name = s.Name
	}
	if s.Min != "" {
		min, err := strconv.ParseFloat(s.Min, 64)
		if err != nil {
			return Setting{}, fmt.Errorf("%s: Min: %w", addr, err)
		}
		setting.Min = &min
	}
	if len(s.Options) > 0 {
		setting.Options = nil
		for _, o := range s.Options {
			v, err := strconv.ParseFloat(o.Value, 64)
			if err != nil {
				return Setting{}, fmt.Errorf("%s: Option %q: %w", addr, o.Value, err)
			}
			setting.Options = append(setting.Options, Option{Value: v, Name: o.Name})
		}
	}
	if f := s.OptionsFromCard; f != nil {
		from := OptionsFrom{Unit: f.Unit}
		if from.Index, err = parseUint16(f.Index); err != nil {
			return Setting{}, fmt.Errorf("%s: OptionsFromCard Index: %w", addr, err)
		}
		if from.CountSubIndex, err = parseUint8(f.CountSubIndex); err != nil {
			return Setting{}, fmt.Errorf("%s: CountSubIndex: %w", addr, err)
		}
		if from.FirstSubIndex, err = parseUint8(f.FirstSubIndex); err != nil {
			return Setting{}, fmt.Errorf("%s: FirstSubIndex: %w", addr, err)
		}
		if from.FirstValue, err = strconv.ParseFloat(f.FirstValue, 64); err != nil {
			return Setting{}, fmt.Errorf("%s: FirstValue: %w", addr, err)
		}
		if from.Divisor, err = parseDivisor(f.Divisor); err != nil {
			return Setting{}, fmt.Errorf("%s: Divisor: %w", addr, err)
		}
		setting.Options = nil
		setting.OptionsFrom = &from
	}
	if f := s.MaxFromCard; f != nil {
		from := MaxFrom{}
		if from.Index, err = parseUint16(f.Index); err != nil {
			return Setting{}, fmt.Errorf("%s: MaxFromCard Index: %w", addr, err)
		}
		if from.SubIndex, err = parseUint8(f.SubIndex); err != nil {
			return Setting{}, fmt.Errorf("%s: MaxFromCard SubIndex: %w", addr, err)
		}
		if from.Divisor, err = parseDivisor(f.Divisor); err != nil {
			return Setting{}, fmt.Errorf("%s: Divisor: %w", addr, err)
		}
		setting.MaxFrom = &from
	}
	for _, w := range s.ShowWhen {
		cond, err := parseCondition(w)
		if err != nil {
			return Setting{}, fmt.Errorf("%s: ShowWhen: %w", addr, err)
		}
		setting.VisibleWhen = append(setting.VisibleWhen, cond)
	}
	return setting, nil
}

func findObject(dict dictionary, index uint16) (dictObject, bool) {
	for _, obj := range dict.Objects {
		if n, err := parseUint16(obj.Index); err == nil && n == index {
			return obj, true
		}
	}
	return dictObject{}, false
}

func findDataType(dict dictionary, name string) (dictDataType, bool) {
	for _, dt := range dict.DataTypes {
		if dt.Name == name {
			return dt, true
		}
	}
	return dictDataType{}, false
}

func parseCondition(c synnaxCondition) (Condition, error) {
	var cond Condition
	var err error
	if cond.Index, err = parseUint16(c.Index); err != nil {
		return cond, err
	}
	if cond.SubIndex, err = parseUint8(c.SubIndex); err != nil {
		return cond, err
	}
	for _, f := range strings.Fields(c.Values) {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return cond, fmt.Errorf("Values: %w", err)
		}
		cond.Values = append(cond.Values, v)
	}
	if c.Above != "" {
		above, err := strconv.ParseFloat(c.Above, 64)
		if err != nil {
			return cond, fmt.Errorf("Above: %w", err)
		}
		cond.Above = &above
	}
	if (len(cond.Values) == 0) == (cond.Above == nil) {
		return cond, fmt.Errorf("needs exactly one of Values and Above")
	}
	return cond, nil
}

// decodeDefault reads the little-endian bytes of an ESI DefaultData as dataType.
func decodeDefault(data string, dataType string) (float64, error) {
	raw, err := hex.DecodeString(data)
	if err != nil {
		return 0, err
	}
	buf := make([]byte, 8)
	copy(buf, raw)
	bits := binary.LittleEndian.Uint64(buf)
	switch dataType {
	case "int8":
		return float64(int8(bits)), nil
	case "uint8":
		return float64(uint8(bits)), nil
	case "int16":
		return float64(int16(bits)), nil
	case "uint16":
		return float64(uint16(bits)), nil
	case "int32":
		return float64(int32(bits)), nil
	case "uint32":
		return float64(uint32(bits)), nil
	case "float32":
		return float64(math.Float32frombits(uint32(bits))), nil
	case "float64":
		return math.Float64frombits(bits), nil
	}
	return 0, fmt.Errorf("unsupported data type %s", dataType)
}

func parseUint16(s string) (uint16, error) {
	v, err := parseHexOrDecimal(s)
	if err != nil {
		return 0, err
	}
	if v > math.MaxUint16 {
		return 0, fmt.Errorf("%s does not fit in 16 bits", s)
	}
	return uint16(v), nil
}

func parseUint8(s string) (uint8, error) {
	v, err := parseHexOrDecimal(s)
	if err != nil {
		return 0, err
	}
	if v > math.MaxUint8 {
		return 0, fmt.Errorf("%s does not fit in 8 bits", s)
	}
	return uint8(v), nil
}

func parseDivisor(s string) (float64, error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	if v <= 0 {
		return 0, fmt.Errorf("must be above 0")
	}
	return v, nil
}

// parseSupplements parses every supplement under dir, sorted by vendor and product.
func parseSupplements(dir string) ([]DeviceSettings, error) {
	var all []DeviceSettings
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".xml") {
			return nil
		}
		settings, err := parseSupplementFile(path)
		if err != nil {
			return err
		}
		all = append(all, settings...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].VendorID != all[j].VendorID {
			return all[i].VendorID < all[j].VendorID
		}
		return all[i].ProductCode < all[j].ProductCode
	})
	for i := 1; i < len(all); i++ {
		if all[i].VendorID == all[i-1].VendorID &&
			all[i].ProductCode == all[i-1].ProductCode {
			return nil, fmt.Errorf(
				"two supplements describe vendor 0x%08X product %d",
				all[i].VendorID,
				all[i].ProductCode,
			)
		}
	}
	return all, nil
}

// writeSettings writes the settings descriptions as a NUL-terminated JSON byte array.
func writeSettings(settings []DeviceSettings, outputPath string) error {
	if settings == nil {
		settings = []DeviceSettings{}
	}
	doc, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	var inc strings.Builder
	inc.WriteString("// AUTO-GENERATED FILE - DO NOT EDIT\n")
	inc.WriteString("// Generated by esi/parser from the supplements directory.\n\n")
	inc.WriteString("const uint8_t REGISTRY_SETTINGS[] = {\n")
	for i, b := range append(doc, 0) {
		if i%16 == 0 {
			inc.WriteString("    ")
		}
		fmt.Fprintf(&inc, "0x%02X,", b)
		if i%16 == 15 {
			inc.WriteString("\n")
		}
	}
	inc.WriteString("\n};\n")
	return os.WriteFile(outputPath, []byte(inc.String()), 0o644)
}
