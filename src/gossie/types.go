package gossie

import (
	"bytes"
	enc "encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

/*
   to do:

   ReversedType

   IntegerType
   DecimalType

   don't assume int is int32 (tho it's prob ok)
   uints, support them?

   maybe add ascii/utf8 types support UUIDType, string native support?
   maybe something better for DateType, instead of just int64 conv (go v1)?
   maybe some more (un)marshalings?

   more error checking, pass along all strconv errors
*/

const (
	_ = iota
	UnknownType
	BytesType
	AsciiType
	UTF8Type
	LongType
	IntegerType
	DecimalType
	UUIDType
	BooleanType
	FloatType
	DoubleType
	DateType
	CounterColumnType
	CompositeType
)

var (
	ErrorUnsupportedMarshaling                  = errors.New("Cannot marshal value")
	ErrorUnsupportedNilMarshaling               = errors.New("Cannot marshal nil")
	ErrorUnsupportedUnmarshaling                = errors.New("Cannot unmarshal value")
	ErrorUnsupportedNativeTypeUnmarshaling      = errors.New("Cannot unmarshal to native type")
	ErrorUnsupportedCassandraTypeUnmarshaling   = errors.New("Cannot unmarshal from Cassandra type")
	ErrorCassandraTypeSerializationUnmarshaling = errors.New("Cassandra serialization is wrong for the type, cannot unmarshal")
)

type TypeDesc int

type UUID [16]byte

func (value UUID) String() string {
	var r []string
	var s int
	for _, size := range [5]int{4, 2, 2, 2, 6} {
		var v int64
		for i := 0; i < size; i++ {
			v = v << 8
			v = v | int64(value[s+i])
		}
		r = append(r, fmt.Sprintf("%0*x", size*2, v))
		s += size
	}
	return strings.Join(r, "-")
}

func NewUUID(value string) (UUID, error) {
	var r []byte
	var ru UUID

	if len(value) != 36 {
		return ru, ErrorUnsupportedMarshaling
	}
	ints := strings.Split(value, "-")
	if len(ints) != 5 {
		return ru, ErrorUnsupportedMarshaling
	}

	for i, size := range [5]int{4, 2, 2, 2, 6} {
		t, err := strconv.ParseInt(ints[i], 16, 64)
		if err != nil {
			return ru, ErrorUnsupportedMarshaling
		}
		b, _ := marshalInt(t, size, BytesType)
		r = append(r, b...)
	}

	unmarshalUUID(r, BytesType, &ru)
	return ru, nil
}

func Marshal(value interface{}, typeDesc TypeDesc) ([]byte, error) {
	// plain nil case
	if value == nil {
		return nil, ErrorUnsupportedNilMarshaling
	}

	// dereference in case we got a pointer, check for nil too
	var dvalue interface{}
	switch v := value.(type) {
	case *[]byte:
		if v == nil {
			return nil, ErrorUnsupportedNilMarshaling
		}
		dvalue = *v
	case *bool:
		if v == nil {
			return nil, ErrorUnsupportedNilMarshaling
		}
		dvalue = *v
	case *int8:
		if v == nil {
			return nil, ErrorUnsupportedNilMarshaling
		}
		dvalue = *v
	case *int16:
		if v == nil {
			return nil, ErrorUnsupportedNilMarshaling
		}
		dvalue = *v
	case *int:
		if v == nil {
			return nil, ErrorUnsupportedNilMarshaling
		}
		dvalue = *v
	case *int32:
		if v == nil {
			return nil, ErrorUnsupportedNilMarshaling
		}
		dvalue = *v
	case *int64:
		if v == nil {
			return nil, ErrorUnsupportedNilMarshaling
		}
		dvalue = *v
	case *string:
		if v == nil {
			return nil, ErrorUnsupportedNilMarshaling
		}
		dvalue = *v
	case *UUID:
		if v == nil {
			return nil, ErrorUnsupportedNilMarshaling
		}
		dvalue = *v
	case *float32:
		if v == nil {
			return nil, ErrorUnsupportedNilMarshaling
		}
		dvalue = *v
	case *float64:
		if v == nil {
			return nil, ErrorUnsupportedNilMarshaling
		}
		dvalue = *v
	default:
		dvalue = v
	}

	switch v := dvalue.(type) {
	case []byte:
		return v, nil
	case bool:
		return marshalBool(v, typeDesc)
	case int8:
		return marshalInt(int64(v), 1, typeDesc)
	case int16:
		return marshalInt(int64(v), 2, typeDesc)
	case int:
		return marshalInt(int64(v), 4, typeDesc)
	case int32:
		return marshalInt(int64(v), 4, typeDesc)
	case int64:
		return marshalInt(v, 8, typeDesc)
	case string:
		return marshalString(v, typeDesc)
	case UUID:
		return marshalUUID(v, typeDesc)
	case float32:
		return marshalFloat32(v, typeDesc)
	case float64:
		return marshalFloat64(v, typeDesc)
	}
	return nil, ErrorUnsupportedMarshaling
}

func marshalBool(value bool, typeDesc TypeDesc) ([]byte, error) {
	switch typeDesc {
	case BytesType, BooleanType:
		b := make([]byte, 1)
		if value {
			b[0] = 1
		}
		return b, nil

	case AsciiType, UTF8Type:
		b := make([]byte, 1)
		if value {
			b[0] = '1'
		} else {
			b[0] = '0'
		}
		return b, nil

	case LongType:
		b := make([]byte, 8)
		if value {
			b[7] = 1
		}
		return b, nil
	}
	return nil, ErrorUnsupportedMarshaling
}

func marshalInt(value int64, size int, typeDesc TypeDesc) ([]byte, error) {
	switch typeDesc {

	case LongType:
		b := make([]byte, 8)
		enc.BigEndian.PutUint64(b, uint64(value))
		return b, nil

	case BytesType:
		b := make([]byte, 8)
		enc.BigEndian.PutUint64(b, uint64(value))
		return b[len(b)-size:], nil

	case DateType:
		if size != 8 {
			return nil, ErrorUnsupportedMarshaling
		}
		b := make([]byte, 8)
		enc.BigEndian.PutUint64(b, uint64(value))
		return b, nil

	case AsciiType, UTF8Type:
		return marshalString(strconv.FormatInt(value, 10), UTF8Type)
	}
	return nil, ErrorUnsupportedMarshaling
}

func marshalString(value string, typeDesc TypeDesc) ([]byte, error) {
	// let cassandra check the ascii-ness of the []byte
	switch typeDesc {
	case BytesType, AsciiType, UTF8Type:
		return []byte(value), nil

	case LongType:
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, err
		}
		return marshalInt(i, 8, LongType)
	}
	return nil, ErrorUnsupportedMarshaling
}

func marshalUUID(value UUID, typeDesc TypeDesc) ([]byte, error) {
	switch typeDesc {
	case BytesType, UUIDType:
		return []byte(value[:]), nil
	}
	return nil, ErrorUnsupportedMarshaling
}

func marshalFloat32(value float32, typeDesc TypeDesc) ([]byte, error) {
	switch typeDesc {
	case BytesType, FloatType:
		var b []byte
		buf := bytes.NewBuffer(b)
		enc.Write(buf, enc.BigEndian, value)
		return buf.Bytes(), nil
		/*
		   case DoubleType:
		       var valueD float64 = float64(value)
		       var b []byte
		       buf := bytes.NewBuffer(b)
		       enc.Write(buf, enc.BigEndian, valueD)
		       return buf.Bytes(), nil
		*/
	}
	return nil, ErrorUnsupportedMarshaling
}

func marshalFloat64(value float64, typeDesc TypeDesc) ([]byte, error) {
	switch typeDesc {
	case BytesType, DoubleType:
		var b []byte
		buf := bytes.NewBuffer(b)
		enc.Write(buf, enc.BigEndian, value)
		return buf.Bytes(), nil
		/*
		   case FloatType:
		       var valueF float32 = float32(value)
		       var b []byte
		       buf := bytes.NewBuffer(b)
		       enc.Write(buf, enc.BigEndian, valueF)
		       return buf.Bytes(), nil
		*/
	}
	return nil, ErrorUnsupportedMarshaling
}

func Unmarshal(b []byte, typeDesc TypeDesc, value interface{}) error {
	switch v := value.(type) {
	case *[]byte:
		*v = b
		return nil
	case *bool:
		return unmarshalBool(b, typeDesc, v)
	case *string:
		return unmarshalString(b, typeDesc, v)
	case *int8:
		return unmarshalInt8(b, typeDesc, v)
	case *int16:
		return unmarshalInt16(b, typeDesc, v)
	case *int:
		var vt int32
		err := unmarshalInt32(b, typeDesc, &vt)
		if err == nil {
			*v = int(vt)
		}
		return err
	case *int32:
		return unmarshalInt32(b, typeDesc, v)
	case *int64:
		return unmarshalInt64(b, typeDesc, v)
	case *UUID:
		return unmarshalUUID(b, typeDesc, v)
	case *float32:
		return unmarshalFloat32(b, typeDesc, v)
	case *float64:
		return unmarshalFloat64(b, typeDesc, v)
	}
	return ErrorUnsupportedNativeTypeUnmarshaling
}

func unmarshalBool(b []byte, typeDesc TypeDesc, value *bool) error {
	switch typeDesc {
	case BytesType, BooleanType:
		if len(b) < 1 {
			return ErrorCassandraTypeSerializationUnmarshaling
		}
		if b[0] == 0 {
			*value = false
		} else {
			*value = true
		}
		return nil

	case AsciiType, UTF8Type:
		if len(b) < 1 {
			return ErrorCassandraTypeSerializationUnmarshaling
		}
		if b[0] == '0' {
			*value = false
		} else {
			*value = true
		}
		return nil

	case LongType:
		if len(b) != 8 {
			return ErrorCassandraTypeSerializationUnmarshaling
		}
		if b[7] == 0 {
			*value = false
		} else {
			*value = true
		}
		return nil
	}
	return ErrorUnsupportedCassandraTypeUnmarshaling
}

func unmarshalInt64(b []byte, typeDesc TypeDesc, value *int64) error {
	switch typeDesc {
	case LongType, BytesType, DateType, CounterColumnType:
		if len(b) != 8 {
			return ErrorCassandraTypeSerializationUnmarshaling
		}
		*value = int64(enc.BigEndian.Uint64(b))
		return nil

	case AsciiType, UTF8Type:
		var r string
		err := unmarshalString(b, AsciiType, &r)
		if err != nil {
			return err
		}
		*value, err = strconv.ParseInt(r, 10, 64)
		if err != nil {
			return err
		}
		return nil
	}
	return ErrorUnsupportedCassandraTypeUnmarshaling
}

func unmarshalInt32(b []byte, typeDesc TypeDesc, value *int32) error {
	switch typeDesc {
	case LongType:
		if len(b) != 8 {
			return ErrorCassandraTypeSerializationUnmarshaling
		}
		*value = int32(enc.BigEndian.Uint64(b))
		return nil

	case BytesType:
		if len(b) != 4 {
			return ErrorCassandraTypeSerializationUnmarshaling
		}
		*value = int32(enc.BigEndian.Uint32(b))
		return nil

	case AsciiType, UTF8Type:
		var r string
		err := unmarshalString(b, AsciiType, &r)
		if err != nil {
			return err
		}
		i, err := strconv.Atoi(r)
		if err != nil {
			return err
		}
		*value = int32(i)
		return nil
	}
	return ErrorUnsupportedCassandraTypeUnmarshaling
}

func unmarshalInt16(b []byte, typeDesc TypeDesc, value *int16) error {
	switch typeDesc {
	case LongType:
		if len(b) != 8 {
			return ErrorCassandraTypeSerializationUnmarshaling
		}
		*value = int16(enc.BigEndian.Uint64(b))
		return nil

	case BytesType:
		if len(b) != 2 {
			return ErrorCassandraTypeSerializationUnmarshaling
		}
		*value = int16(enc.BigEndian.Uint16(b))
		return nil

	case AsciiType, UTF8Type:
		var r string
		err := unmarshalString(b, AsciiType, &r)
		if err != nil {
			return err
		}
		i, err := strconv.Atoi(r)
		if err != nil {
			return err
		}
		*value = int16(i)
		return nil
	}
	return ErrorUnsupportedCassandraTypeUnmarshaling
}

func unmarshalInt8(b []byte, typeDesc TypeDesc, value *int8) error {
	switch typeDesc {
	case LongType:
		if len(b) != 8 {
			return ErrorCassandraTypeSerializationUnmarshaling
		}
		*value = int8(b[7])
		return nil

	case BytesType:
		if len(b) != 1 {
			return ErrorCassandraTypeSerializationUnmarshaling
		}
		*value = int8(b[0])
		return nil

	case AsciiType, UTF8Type:
		var r string
		err := unmarshalString(b, AsciiType, &r)
		if err != nil {
			return err
		}
		i, err := strconv.Atoi(r)
		if err != nil {
			return err
		}
		*value = int8(i)
		return nil
	}
	return ErrorUnsupportedCassandraTypeUnmarshaling
}

func unmarshalString(b []byte, typeDesc TypeDesc, value *string) error {
	switch typeDesc {
	case BytesType, AsciiType, UTF8Type:
		*value = string(b)
		return nil

	case LongType:
		var i int64
		err := unmarshalInt64(b, LongType, &i)
		if err != nil {
			return err
		}
		*value = strconv.FormatInt(i, 10)
		return nil
	}
	return ErrorUnsupportedCassandraTypeUnmarshaling
}

func unmarshalUUID(b []byte, typeDesc TypeDesc, value *UUID) error {
	switch typeDesc {
	case BytesType, UUIDType:
		if len(b) != 16 {
			return ErrorCassandraTypeSerializationUnmarshaling
		}
		copy((*value)[:], b)
		return nil
	}
	return ErrorUnsupportedCassandraTypeUnmarshaling
}

func unmarshalFloat32(b []byte, typeDesc TypeDesc, value *float32) error {
	switch typeDesc {
	case BytesType, FloatType:
		buf := bytes.NewBuffer(b)
		enc.Read(buf, enc.BigEndian, value)
		return nil
		/*
		   case DoubleType:
		       var valueD float64
		       buf := bytes.NewBuffer(b)
		       enc.Read(buf, enc.BigEndian, &valueD)
		       *value = float32(valueD)
		       return nil
		*/
	}
	return ErrorUnsupportedCassandraTypeUnmarshaling
}

func unmarshalFloat64(b []byte, typeDesc TypeDesc, value *float64) error {
	switch typeDesc {
	case BytesType, DoubleType:
		buf := bytes.NewBuffer(b)
		enc.Read(buf, enc.BigEndian, value)
		return nil
		/*
		   case FloatType:
		       var valueF float32
		       buf := bytes.NewBuffer(b)
		       enc.Read(buf, enc.BigEndian, &valueF)
		       *value = float64(valueF)
		       return nil
		*/
	}
	return ErrorUnsupportedCassandraTypeUnmarshaling
}

type TypeClass struct {
	Desc       TypeDesc
	Components []TypeClass
}

func parseTypeDesc(cassType string) TypeDesc {
	switch cassType {
	case "BytesType", "org.apache.cassandra.db.marshal.BytesType":
		return BytesType
	case "AsciiType", "org.apache.cassandra.db.marshal.AsciiType":
		return AsciiType
	case "UTF8Type", "org.apache.cassandra.db.marshal.UTF8Type":
		return UTF8Type
	case "LongType", "org.apache.cassandra.db.marshal.LongType":
		return LongType
	case "IntegerType", "org.apache.cassandra.db.marshal.IntegerType":
		return IntegerType
	case "DecimalType", "org.apache.cassandra.db.marshal.DecimalType":
		return DecimalType
	case "UUIDType", "org.apache.cassandra.db.marshal.UUIDType":
		return UUIDType
	case "BooleanType", "org.apache.cassandra.db.marshal.BooleanType":
		return BooleanType
	case "FloatType", "org.apache.cassandra.db.marshal.FloatType":
		return FloatType
	case "DoubleType", "org.apache.cassandra.db.marshal.DoubleType":
		return DoubleType
	case "DateType", "org.apache.cassandra.db.marshal.DateType":
		return DateType
	case "CounterColumnType", "org.apache.cassandra.db.marshal.CounterColumnType":
		return CounterColumnType
	}
	return BytesType
}

func parseTypeClass(cassType string) TypeClass {
	r := TypeClass{Desc: BytesType}

	// check for composite and parse it
	if strings.HasPrefix(cassType, "org.apache.cassandra.db.marshal.CompositeType(") {
		r.Desc = CompositeType
		componentsString := cassType[strings.Index(cassType, "(")+1 : len(cassType)-1]
		componentsSlice := strings.Split(componentsString, ",")
		var components []TypeClass
		for _, component := range componentsSlice {
			components = append(components, parseTypeClass(component))
		}
		r.Components = components
		return r
	}

	r.Desc = parseTypeDesc(cassType)

	return r
}

func packComposite(current, component []byte, comparator, sliceStart, inclusive bool) []byte {
	var eoc byte = 0
	if comparator {
		if inclusive {
			if sliceStart {
				eoc = 0xff
			} else {
				eoc = 0x01
			}
		} else {
			if sliceStart {
				eoc = 0x01
			} else {
				eoc = 0xff
			}
		}
	}
	r := make([]byte, 2)
	enc.BigEndian.PutUint16(r, uint16(len(component)))
	r = append(current, r...)
	r = append(r, component...)
	return append(r, eoc)
}

func unpackComposite(composite []byte) [][]byte {
	components := make([][]byte, 0)
	for len(composite) > 0 {
		l := enc.BigEndian.Uint16(composite[:2])
		components = append(components, composite[2:2+l])
		composite = composite[3+l:]
	}
	return components
}
