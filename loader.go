package tojvm

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

type Class struct {
	ConstPool  ConstPool
	Name       string
	Super      string
	Flags      uint16
	Interfaces []string
	Fields     []Field
	Methods    []Field
	Attributes []Attribute
}

type Tag byte

// From Table 4.4-A
const (
	TagClass              Tag = 7
	TagFieldRef               = 9
	TagMethodRef              = 10
	TagInterfaceMethodRef     = 11
	TagString                 = 8
	TagInteger                = 3
	TagFloat                  = 4
	TagLong                   = 5
	TagDouble                 = 6
	TagNameAndType            = 12
	TagUTF8                   = 1
	TagMethodHandle           = 15
	TagMethodType             = 16
	TagInvokeDynamic          = 18
)

type Const struct {
	Tag              Tag
	NameIndex        uint16
	ClassIndex       uint16
	NameAndTypeIndex uint16
	StringIndex      uint16
	DescIndex        uint16
	Integer          int32
	Long             int64
	Float            float32
	Double           float64
	String           string
}

type Field struct {
	Flags      uint16
	Name       string
	Descriptor string
	Attributes []Attribute
}

type Attribute struct {
	Name string
	Data []byte
}

type ConstPool []Const

func (cp ConstPool) Resolve(index uint16) string {
	if cp[index-1].Tag == TagUTF8 {
		return cp[index-1].String
	} else if cp[index-1].Tag == TagString {
		return cp.Resolve(cp[index-1].StringIndex)
	} else if cp[index-1].Tag == TagClass || cp[index-1].Tag == TagNameAndType {
		return cp.Resolve(cp[index-1].NameIndex)
	}
	return ""
}

type loader struct {
	r   io.Reader
	err error
}

func (l *loader) bytes(n int) []byte {
	b := make([]byte, n, n)
	if l.err == nil {
		_, l.err = io.ReadFull(l.r, b)
	}
	return b
}
func (l *loader) u1() uint8  { return l.bytes(1)[0] }
func (l *loader) u2() uint16 { return binary.BigEndian.Uint16(l.bytes(2)) }
func (l *loader) u4() uint32 { return binary.BigEndian.Uint32(l.bytes(4)) }
func (l *loader) u8() uint64 { return binary.BigEndian.Uint64(l.bytes(8)) }

func (l *loader) cpinfo() (constPool ConstPool) {
	constPoolCount := l.u2()
	for i := uint16(1); i < constPoolCount; i++ {
		c := Const{Tag: Tag(l.u1())}
		switch c.Tag {
		case TagClass:
			c.NameIndex = l.u2()
		case TagFieldRef, TagMethodRef, TagInterfaceMethodRef:
			c.ClassIndex = l.u2()
			c.NameAndTypeIndex = l.u2()
		case TagString:
			c.StringIndex = l.u2()
		case TagInteger:
			c.Integer = int32(l.u4())
		case TagFloat:
			c.Float = math.Float32frombits(l.u4())
		case TagLong:
			c.Long = int64(l.u8())
		case TagDouble:
			c.Double = math.Float64frombits(l.u8())
		case TagNameAndType:
			c.NameIndex, c.DescIndex = l.u2(), l.u2()
		case TagUTF8:
			c.String = string(l.bytes(int(l.u2())))
		default:
			l.err = fmt.Errorf("unsupported tag: %d", c.Tag)
		}
		constPool = append(constPool, c)
		if c.Tag == TagDouble || c.Tag == TagLong {
			// For 64-bit types an additional, valid, but unused const should be added
			constPool = append(constPool, Const{Tag: TagInteger})
			i++
		}
	}
	return constPool
}

func (l *loader) interfaces(cp ConstPool) (interfaces []string) {
	interfaceCount := l.u2()
	for i := uint16(0); i < interfaceCount; i++ {
		interfaces = append(interfaces, cp.Resolve(l.u2()))
	}
	return interfaces
}

func (l *loader) fields(cp ConstPool) (fields []Field) {
	fieldsCount := l.u2()
	for i := uint16(0); i < fieldsCount; i++ {
		fields = append(fields, Field{
			Flags:      l.u2(),
			Name:       cp.Resolve(l.u2()),
			Descriptor: cp.Resolve(l.u2()),
			Attributes: l.attrs(cp),
		})
	}
	return fields
}

func (l *loader) attrs(cp ConstPool) (attrs []Attribute) {
	attributesCount := l.u2()
	for i := uint16(0); i < attributesCount; i++ {
		attrs = append(attrs, Attribute{
			Name: cp.Resolve(l.u2()),
			Data: l.bytes(int(l.u4())),
		})
	}
	return attrs
}

func Load(r io.Reader) (Class, error) {
	loader := &loader{r: r}
	c := Class{}
	loader.u8()           // magic, minor, major
	cp := loader.cpinfo() // const pool info
	c.ConstPool = cp
	c.Flags = loader.u2()             // access flags
	c.Name = cp.Resolve(loader.u2())  // this class
	c.Super = cp.Resolve(loader.u2()) // super class
	c.Interfaces = loader.interfaces(cp)
	c.Fields = loader.fields(cp)    // fields
	c.Methods = loader.fields(cp)   // methods
	c.Attributes = loader.attrs(cp) // methods
	return c, loader.err
}
