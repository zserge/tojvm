package tojvm

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
)

type Value interface{}

type Frame struct {
	Class  *Object
	IP     uint32
	Code   []byte
	Locals []Value
	Stack  []Value
}

func (f *Frame) push(v Value) {
	f.Stack = append(f.Stack, v)
}

func (f *Frame) pop() Value {
	v := f.Stack[len(f.Stack)-1]
	f.Stack = f.Stack[:len(f.Stack)-1]
	return v
}

type Object struct {
	Class
	ClassInstance *Object
	SuperInstance *Object
	Fields        map[string]Value
}

func (o *Object) New() *Object {
	return &Object{
		Class:         o.Class,
		ClassInstance: o,
		Fields:        map[string]Value{},
	}
}

func (o *Object) Const(index uint16) Value {
	return o.ConstPool.Resolve(index)
}

func (o *Object) Field(name string) Value {
	return o.Fields[name]
}

func (o *Object) SetField(name string, value Value) {
	o.Fields[name] = value
}

func (o *Object) Method(name, desc string) (Field, error) {
	for _, m := range o.Methods {
		if m.Name == name && (desc == "" || desc == m.Descriptor) {
			return m, nil
		}
	}
	return Field{}, errors.New("method not found")
}

type VM struct {
	ClassPath []string
	Classes   []*Object
	Native    map[string]func(...Value) Value
}

func New(classPath ...string) *VM {
	vm := &VM{
		ClassPath: classPath,
		Classes: []*Object{
			&Object{
				Class: Class{
					Name:    "java/lang/Object",
					Methods: []Field{{Name: "<init>", Descriptor: "()V"}},
				},
			},
		},
		Native: map[string]func(...Value) Value{},
	}
	vm.RegisterNative("java/lang/Object", "<init>", "()V", func(...Value) Value {
		return nil
	})
	return vm
}

func (vm *VM) RegisterNative(class, method, desc string, f func(...Value) Value) {
	vm.Native[class+"."+method] = f
}

func (vm *VM) Class(name string) (*Object, error) {
	for _, c := range vm.Classes {
		if c.Name == name {
			return c, nil
		}
	}
	for _, path := range vm.ClassPath {
		f, err := os.Open(filepath.Join(path, name+".class"))
		if err != nil {
			continue
		}
		c, err := Load(f)
		f.Close()
		if err != nil {
			continue
		}
		var super *Object
		if c.Super != "" {
			super, err = vm.Class(c.Super)
			if err != nil {
				return nil, err
			}
		}
		classObj := &Object{
			Class:         c,
			SuperInstance: super,
			Fields:        map[string]Value{},
		}
		vm.Classes = append(vm.Classes, classObj)
		if m, err := classObj.Method("<clinit>", "()V"); err == nil {
			if _, err := vm.callMethod(classObj, m); err != nil {
				return nil, err
			}
		}
		return classObj, nil
	}
	return nil, errors.New("class not found")
}

func (vm *VM) Call(class, method string, args ...Value) (Value, error) {
	c, err := vm.Class(class)
	if err != nil {
		return nil, err
	}
	m, err := c.Method(method, "")
	if err != nil {
		return nil, err
	}
	return vm.callMethod(c, m, args...)
}

func argc(desc string) (n int) {
	inClass := false
	for i := 1; i < len(desc); i++ {
		if inClass {
			if desc[i] == ';' {
				inClass = false
			}
			continue
		}
		if desc[i] == ')' {
			return n
		} else if desc[i] == 'L' {
			inClass = true
		}
		n++
	}
	return 0
}

func (vm *VM) CallMethod(obj *Object, method, desc string, args ...Value) (Value, error) {
	m, err := obj.Method(method, desc)
	if err != nil {
		return nil, err
	}
	return vm.callMethod(obj, m, args...)
}

func (vm *VM) callMethod(obj *Object, m Field, args ...Value) (Value, error) {
	for _, a := range m.Attributes {
		if a.Name == "Code" && len(a.Data) > 8 {
			maxLocals := binary.BigEndian.Uint16(a.Data[2:4])
			frame := Frame{
				Class:  obj,
				Code:   a.Data[8:],
				Locals: make([]Value, maxLocals, maxLocals),
			}
			for i := 0; i < len(args); i++ {
				frame.Locals[i] = args[i]
			}
			return vm.exec(frame)
		}
	}
	f, ok := vm.Native[obj.Name+"."+m.Name]
	if ok {
		return f(args...), nil
	}
	return nil, errors.New("method code not found")
}

func (vm *VM) exec(frame Frame) (Value, error) {
	for {
		op := frame.Code[frame.IP]
		//log.Printf("%02x %v", op, frame.Stack)
		switch op {
		//
		// Constants
		//
		case 0x00: // NOP
		case 0x01: // ACONST_NULL
			frame.push(nil)
		case 0x02: // ICONST_M1
			frame.push(int32(-1))
		case 0x03: // ICONST_0
			frame.push(int32(0))
		case 0x04: // ICONST_1
			frame.push(int32(1))
		case 0x05: // ICONST_2
			frame.push(int32(2))
		case 0x06: // ICONST_3
			frame.push(int32(3))
		case 0x07: // ICONST_4
			frame.push(int32(4))
		case 0x08: // ICONST_5
			frame.push(int32(5))
		case 0x09: // LCONST_0
			frame.push(int64(5))
		case 0x0A: // LCONST_1
			frame.push(int64(1))
		case 0x0B: // FCONST_0
			frame.push(float32(0))
		case 0x0C: // FCONST_1
			frame.push(float32(1))
		case 0x0D: // FCONST_2
			frame.push(float32(2))
		case 0x0E: // DCONST_0
			frame.push(0.0)
		case 0x0F: // DCONST_1
			frame.push(1.0)
		case 0x10: // BIPUSH
			frame.push(int8(frame.Code[frame.IP+1]))
			frame.IP = frame.IP + 1
		case 0x11: // SIPUSH
			frame.push(int16(binary.BigEndian.Uint16(frame.Code[frame.IP+1:])))
			frame.IP = frame.IP + 2
		case 0x12: // LDC
			frame.push(frame.Class.Const(uint16(frame.Code[frame.IP+1])))
			frame.IP = frame.IP + 1
		case 0x13, 0x14: // LDC_W, LDC2_W
			frame.push(frame.Class.Const(uint16(frame.Code[frame.IP+1])))
			frame.IP = frame.IP + 1

		//
		// Loads
		//
		case 0x15, 0x16, 0x17, 0x18, 0x19: // ILOAD, LLOAD, FLOAD, DLOAD, ALOAD
			frame.push(frame.Locals[frame.Code[frame.IP+1]])
			frame.IP = frame.IP + 1
		case 0x1A, 0x1E, 0x22, 0x26, 0x2A: // ILOAD_0, LLOAD_0, FLOAD_0, DLOAD_0, ALOAD_0
			frame.push(frame.Locals[0])
		case 0x1B, 0x1F, 0x23, 0x27, 0x2B: // ILOAD_1, LLOAD_1, FLOAD_1, DLOAD_1, ALOAD_1
			frame.push(frame.Locals[1])
		case 0x1C, 0x20, 0x24, 0x28, 0x2C: // ILOAD_2, LLOAD_2, FLOAD_2, DLOAD_2, ALOAD_2
			frame.push(frame.Locals[2])
		case 0x1D, 0x21, 0x25, 0x29, 0x2D: // ILOAD_3, LLOAD_3, FLOAD_3, DLOAD_3, ALOAD_3
			frame.push(frame.Locals[3])
		case 0x2E, 0x2F, 0x30, 0x31, 0x32, 0x33, 0x34, 0x35: // IALOAD, LALOAD, FALOAD, DALOAD, AALOAD, BALOAD, CALOAD, SALOAD
			a := frame.pop().([]Value)
			i := frame.pop().(int32) // XXX other index types?
			frame.push(a[i])

		//
		// Stores
		//
		case 0x36: // ISTORE
		case 0x37: // LSTORE
		case 0x38: // FSTORE
		case 0x39: // DSTORE
		case 0x3A: // ASTORE
		case 0x3B: // ISTORE_0
		case 0x3C: // ISTORE_1
		case 0x3D: // ISTORE_2
		case 0x3E: // ISTORE_3
		case 0x3F: // LSTORE_0
		case 0x40: // LSTORE_1
		case 0x41: // LSTORE_2
		case 0x42: // LSTORE_3
		case 0x43: // LSTORE_3
		//...
		case 0x4A: // DSTORE_3
		case 0x4B: // ASTORE_0
		case 0x4C: // ASTORE_1
		case 0x4D: // ASTORE_2
		case 0x4E: // ASTORE_3
		case 0x4F: // IASTORE
		//...
		case 0x53: // AASTORE

		//
		// Stack
		//
		case 0x57: // POP
		case 0x59: // DUP
			value := frame.pop()
			frame.push(value)
			frame.push(value)
		case 0x5F: // SWAP
			a := frame.pop()
			b := frame.pop()
			frame.push(a)
			frame.push(b)

		//
		// Math
		//
		case 0x60: // IADD
			frame.push(frame.pop().(int32) + frame.pop().(int32))
		case 0x61: // LADD
			frame.push(frame.pop().(int64) + frame.pop().(int64))
		case 0x62: // FADD
			frame.push(frame.pop().(float32) + frame.pop().(float32))
		case 0x63: // DADD
			frame.push(frame.pop().(float64) + frame.pop().(float64))
		case 0x64: // ISUB
			a, b := frame.pop().(int32), frame.pop().(int32)
			frame.push(b - a)
		case 0x65: // LSUB
			a, b := frame.pop().(int64), frame.pop().(int64)
			frame.push(b - a)
		case 0x66: // FSUB
			a, b := frame.pop().(float32), frame.pop().(float32)
			frame.push(b - a)
		case 0x67: // DSUB
			a, b := frame.pop().(float64), frame.pop().(float64)
			frame.push(b - a)
		case 0x68: // IMUL
			frame.push(frame.pop().(int32) * frame.pop().(int32))
		case 0x69: // LMUL
			frame.push(frame.pop().(int64) * frame.pop().(int64))
		case 0x6A: // FMUL
			frame.push(frame.pop().(float32) * frame.pop().(float32))
		case 0x6B: // DMUL
			frame.push(frame.pop().(float64) * frame.pop().(float64))
		case 0x6F: // DDIV
		case 0x70: // IREM
		case 0x84: // IINC

		//
		// Conversions
		//
		case 0x87: // I2D
		case 0x92: // I2C

		//
		// Comparisons
		//
		case 0x98: // DCMPG
		case 0x9A: // IFNE
		case 0x9B: // IFLT
		case 0x9C: // IFGE
		case 0x9E: // IFLE
		case 0xA1: // IF_ICMPLT
		case 0xA2: // IF_ICMPGE
		case 0xA3: // IF_ICMPGT
		case 0xA4: // IF_ICMPLE

		//
		// Controls
		//
		case 0xA7: // GOTO
			branch := uint32(binary.BigEndian.Uint16(frame.Code[frame.IP+1:]))
			frame.IP = frame.IP - 3 + branch
		case 0xA8: // JSR
		case 0xA9: // RET
		case 0xAC, 0xAD, 0xAE, 0xAF, 0xB0: // IRETURN, LRETURN, FRETURN, DRETURN, ARETURN
			return frame.pop(), nil
		case 0xB1: // RETURN
			return nil, nil

		//
		// References
		//
		case 0xB2, 0xB3, 0xB4, 0xB5, 0xB6, 0xB7, 0xB8:
			cp := frame.Class.ConstPool
			index := uint16(binary.BigEndian.Uint16(frame.Code[frame.IP+1:]))
			frame.IP = frame.IP + 2
			ref := cp[index-1]
			className := cp.Resolve(ref.ClassIndex)
			name := cp.Resolve(cp[ref.NameAndTypeIndex-1].NameIndex)
			desc := cp.Resolve(cp[ref.NameAndTypeIndex-1].DescIndex)
			c, err := vm.Class(className)
			if err != nil {
				return nil, err
			}
			switch op {
			case 0xB2: // GETSTATIC
				frame.push(c.Field(name))
			case 0xB3: // PUTSTATIC
				c.SetField(name, frame.pop())
			case 0xB4: // GETFIELD
				obj := frame.pop().(*Object)
				frame.push(obj.Field(name))
			case 0xB5: // PUTFIELD
				value := frame.pop()
				obj := frame.pop().(*Object)
				obj.SetField(name, value)
			case 0xB6: // INVOKEVIRTUAL
				n := argc(desc)
				res, err := vm.CallMethod(c, name, desc, frame.Stack[len(frame.Stack)-n-1:]...)
				if err != nil {
					return nil, err
				}
				frame.Stack = frame.Stack[:len(frame.Stack)-n]
				_ = res
			case 0xB7: // INVOKESPECIAL
				n := argc(desc)
				res, err := vm.CallMethod(c, name, desc, frame.Stack[len(frame.Stack)-n-1:]...)
				if err != nil {
					return nil, err
				}
				frame.Stack = frame.Stack[:len(frame.Stack)-n]
				_ = res
			case 0xB8: // INVOKESTATIC
				n := argc(desc)
				res, err := vm.CallMethod(c, name, desc, frame.Stack[len(frame.Stack)-n:]...)
				if err != nil {
					return nil, err
				}
				frame.Stack = frame.Stack[:len(frame.Stack)-n]
				_ = res
			}
		case 0xB9: // INVOKEINTERFACE
		case 0xBA: // INVOKEDYNAMIC
		case 0xBB: // NEW
			cp := frame.Class.ConstPool
			index := uint16(binary.BigEndian.Uint16(frame.Code[frame.IP+1:]))
			frame.IP = frame.IP + 2
			className := cp.Resolve(cp[index-1].NameIndex)
			c, err := vm.Class(className)
			if err != nil {
				return nil, err
			}
			obj := c.New()
			frame.push(obj)
		case 0xBC: // NEWARRAY
		case 0xBD: // ANEWARRAY
		case 0xBE: // ARRAYLENGTH
		}
		frame.IP++
	}
}
