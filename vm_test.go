package tojvm

import (
	"log"
	"os"
	"testing"
)

func runtimeLog(args ...Value) Value {
	va := []interface{}{}
	for _, a := range args {
		va = append(va, a.(interface{}))
	}
	log.Println(va...)
	return nil
}

func TestLoader(t *testing.T) {
	f, err := os.Open("testdata/FieldsAndMethods.class")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	c, err := Load(f)
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "FieldsAndMethods" || c.Super != "java/lang/Object" {
		t.Error(c.Name, c.Super)
	}
	if len(c.Fields) != 2 {
		t.Error(c.Fields)
	}
	if len(c.Methods) != 8 {
		t.Error(c.Methods)
	}
}

func TestAdd(t *testing.T) {
	vm := New("testdata")
	if res, err := vm.Call("FieldsAndMethods", "add", int32(2), int32(3)); err != nil {
		t.Error(err)
	} else if n, ok := res.(int32); !ok || n != int32(5) {
		t.Error(res)
	}
}

func TestHello(t *testing.T) {
	vm := New("testdata")
	vm.RegisterNative("Runtime", "log", "(Ljava/lang/String;)V", runtimeLog)
	if res, err := vm.Call("FieldsAndMethods", "hello"); err != nil {
		t.Error(err)
	} else if res != nil {
		t.Error(res)
	}
}

func TestStaticFields(t *testing.T) {
	vm := New("testdata")
	for i := 0; i < 3; i++ {
		if res, err := vm.Call("FieldsAndMethods", "incrementB"); err != nil {
			t.Error(err)
		} else if res != nil {
			t.Error(res)
		}
	}
	if c, err := vm.Class("FieldsAndMethods"); err != nil {
		t.Error(err)
	} else if c.Fields["b"].(int32) != int32(5) {
		t.Error(c.Fields)
	}
}

func TestInstanceFields(t *testing.T) {
	vm := New("testdata")
	res, err := vm.Call("FieldsAndMethods", "create")
	if res == nil || err != nil {
		t.Error(res, err)
	}
	obj := res.(*Object)
	if obj.Fields["a"].(int32) != int32(1) {
		t.Error(obj.Fields)
	}
	vm.Call("FieldsAndMethods", "incrementA", obj)
	vm.Call("FieldsAndMethods", "incrementA", obj)
	vm.Call("FieldsAndMethods", "incrementA", obj)
	if obj.Fields["a"].(int32) != int32(4) {
		t.Error(obj.Fields)
	}
}
