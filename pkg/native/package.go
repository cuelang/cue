package native

import (
	encodingjson "encoding/json"
	"fmt"
	"go/ast"
	"reflect"
	"strconv"

	"cuelang.org/go/cue"
	cueast "cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/literal"
	"cuelang.org/go/encoding/json"
	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/pkg/internal"
)

type Package interface {
	// ImportPath name of Package
	//
	// Exported Methods
	//
	// Func:
	// 		func () SomeMethod(arg ...interface{}) (...interface{}, error)
	// Const:
	//      func () Const() interface{}
	//
	// Func params will convert from cue.Value,
	// struct values need json tag for unmarshalling
	ImportPath() string
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

func newInternalPackage(pkg Package) *internal.Package {
	p := &internal.Package{}

	importPath := pkg.ImportPath()

	rv := reflect.ValueOf(pkg)
	t := rv.Type()

	for i := 0; i < rv.NumMethod(); i++ {
		m := rv.Method(i)
		mt := t.Method(i)
		name := mt.Name

		if !ast.IsExported(name) || name == "ImportPath" {
			continue
		}

		fnT := m.Type()

		switch fnT.NumOut() {
		case 2:
			if fnT.NumIn() > 0 {
				if err := fnT.Out(1); err.Kind() == reflect.Interface && err.AssignableTo(errorType) {

					params := make([]internal.Param, fnT.NumIn())
					inputs := make([]reflect.Type, len(params))

					for i := range params {
						in := fnT.In(i)

						params[i] = internal.Param{Kind: adtKindFromReflectType(in)}
						inputs[i] = in
					}

					resultKind := adtKindFromReflectType(fnT.Out(0))

					builtin := &internal.Builtin{
						Name:   name,
						Params: params,
						Result: resultKind,
						Func: func(c *internal.CallCtxt) {
							values := make([]reflect.Value, len(params))

							for i := range values {
								rv := reflect.New(inputs[i]).Elem()

								cueValue := c.Value(i)

								if err := setValue(rv, cueValue); err != nil {
									c.Err = errors.Wrapf(err, c.Pos(), "parameter %d of %s.%s should be %s, but got %s", i, importPath, name, params[i], cueValue.Kind())
									return
								}

								values[i] = rv
							}

							if c.Do() {
								outputs := m.Call(values)

								if resultKind == adt.ScalarKinds {
									c.Ret, c.Err = outputs[0].Interface(), outputs[1].Interface()
								} else {
									c.Err = outputs[1].Interface()
									if c.Err == nil {
										ret, err := cueAstExprFromGoValues(outputs[0].Interface())
										if err != nil {
											c.Err = err
										} else {
											c.Ret = ret
										}
									}
								}
							}
						},
					}

					p.Native = append(p.Native, builtin)
				}
			}
		case 1:
			if fnT.NumIn() == 0 {
				outputs := m.Call(nil)[0]

				p.Native = append(p.Native, &internal.Builtin{
					Name:  name,
					Const: cueLitFromGoValue(outputs.Interface()),
				})
			}
		}
	}

	return p
}

func cueLitFromGoValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return literal.String.Quote(v)
	case []byte:
		return literal.Bytes.Quote(string(v))
	case int:
		return strconv.FormatInt(int64(v), 10)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case fmt.Stringer:
		return v.String()
	case bool:
		return strconv.FormatBool(v)
	default:
		panic(fmt.Errorf("unsupported value %#v", v))
	}
}

func cueAstExprFromGoValues(v interface{}) (cueast.Expr, error) {
	data, err := encodingjson.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.Extract("v", data)
}

func setValue(rv reflect.Value, v cue.Value) error {
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		return setValue(rv.Elem(), v)
	}

	switch v.Kind() {
	case adt.TopKind, adt.BottomKind:
		return setValue(rv, v.Eval())
	case adt.NullKind:
		return nil
	case adt.BoolKind:
		b, err := v.Bool()
		if err != nil {
			return err
		}
		rv.SetBool(b)
		return nil
	case adt.IntKind:
		b, err := v.Int64()
		if err != nil {
			return err
		}
		rv.SetInt(b)
		return nil
	case adt.BytesKind:
		b, err := v.Bytes()
		if err != nil {
			return err
		}
		rv.SetBytes(b)
		return nil
	case adt.StringKind:
		b, err := v.String()
		if err != nil {
			return err
		}
		rv.SetString(b)
		return nil
	case adt.FloatKind:
		b, err := v.Float64()
		if err != nil {
			return err
		}
		rv.SetFloat(b)
	case adt.ListKind, adt.StructKind:
		// todo need v.Values()
		bytes, err := v.MarshalJSON()
		if err != nil {
			return err
		}
		return encodingjson.Unmarshal(bytes, rv.Addr().Interface())
	}

	return fmt.Errorf("unsupport cue value %s %T", v.Kind(), v)
}

func adtKindFromReflectType(t reflect.Type) adt.Kind {
	switch t.Kind() {
	case reflect.Ptr:
		return adt.NullKind | adtKindFromReflectType(t.Elem())
	case reflect.String:
		return adt.StringKind
	case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
		return adt.IntKind
	case reflect.Uint, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8:
		return adt.IntKind
	case reflect.Float32, reflect.Float64:
		return adt.FloatKind
	case reflect.Bool:
		return adt.BoolKind
	case reflect.Array:
		// byte
		if t.Elem().Kind() == reflect.Int8 {
			return adt.BytesKind
		}
		return adt.ListKind
	case reflect.Struct, reflect.Map:
		return adt.StructKind
	}
	return adt.TopKind
}
