package tools

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/modern-go/reflect2"
)

func DoTagFunc(v interface{}, data interface{}, fn []func(reflect.StructField, reflect.Value, interface{}) error) (err error) {
	if reflect2.IsNil(v) {
		return
	}

	// default tag处理
	vType := reflect2.TypeOf(v)
	vType1 := vType.Type1()

	switch vType1.Kind() {
	case reflect.Interface, reflect.Ptr:
	default:
		// interface能生效
		return
	}

	indirect := reflect.Indirect(reflect.ValueOf(v))
	for i := 0; i < indirect.NumField(); i++ {
		field := indirect.Field(i)
		fieldStruct := vType1.Elem().Field(i)

		for _, f := range fn {
			if err = f(fieldStruct, field, data); err != nil {
				return
			}
		}
	}

	return
}

func SetDefaultValueIfNil(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	if !vValue.CanSet() {
		return
	}
	structTag := structField.Tag
	if containTag(structTag, "default") || vValue.Kind() == reflect.Struct || vValue.Kind() == reflect.Ptr {
		switch vValue.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if vValue.Int() == 0 {
				v, _ := strconv.Atoi(structTag.Get("default"))
				vValue.SetInt(int64(v))
			}
		case reflect.String:
			if vValue.String() == "" {
				vValue.SetString(structTag.Get("default"))
			}
		case reflect.Float32:
			if vValue.Float() == 0 {
				v, _ := strconv.ParseFloat(structTag.Get("default"), 32)
				vValue.SetFloat(v)
			}
		case reflect.Float64:
			if vValue.Float() == 0 {
				v, _ := strconv.ParseFloat(structTag.Get("default"), 64)
				vValue.SetFloat(v)
			}
		case reflect.Struct:
			t := structField.Type
			for i := 0; i < t.NumField(); i++ {
				fieldStruct := t.Field(i)
				if err = SetDefaultValueIfNil(fieldStruct, vValue.Field(i), data); err != nil {
					return
				}
			}
		case reflect.Ptr:
			pValue := reflect.New(structField.Type.Elem()).Elem()
			switch pValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				if vValue.IsNil() {
					if v := structTag.Get("default"); v != "" {
						i, _ := strconv.Atoi(v)
						vValue.Set(reflect.ValueOf(&i))
					}
				}
			case reflect.String:
				if vValue.IsNil() {
					if v := structTag.Get("default"); v != "" {
						vValue.Set(reflect.ValueOf(&v))
					}
				}
			case reflect.Float32, reflect.Float64:
				if vValue.IsNil() {
					if v := structTag.Get("default"); v != "" {
						f, _ := strconv.ParseFloat(v, 64)
						vValue.Set(reflect.ValueOf(&f))
					}
				}
			case reflect.Bool:
				if vValue.IsNil() {
					if v := structTag.Get("default"); v != "" {
						b, _ := strconv.ParseBool(v)
						vValue.Set(reflect.ValueOf(&b))
					}
				}
			case reflect.Ptr:
				fmt.Println("ptr ptr no support Func[SetDefaultValueIfNil]")
			case reflect.Struct:
				if vValue.IsZero() {
					return
				}
				for i := 0; i < pValue.NumField(); i++ {
					field := vValue.Elem().Field(i)
					fieldStruct := pValue.Type().Field(i)
					if err = SetDefaultValueIfNil(fieldStruct, field, data); err != nil {
						return
					}
				}
			default:
			}
		case reflect.Bool:
			fmt.Println("bool no support Func[SetDefaultValueIfNil]")
		default:

		}
	}

	return
}

func SetParam(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	if !vValue.CanSet() {
		return
	}

	var params []string
	m := data.(map[string][]string)
	structTag := structField.Tag
	if containTag(structTag, "param") {
		params = m[structTag.Get("param")]
	} else {
		name := structField.Name
		// 首字母变小
		lName := strings.ToLower(name[:1]) + name[1:]
		for k, v := range m {
			if ToCamel(k) == lName || name == k {
				params = v
				break
			}
		}
	}

	if len(params) > 0 {
		switch vValue.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			v, _ := strconv.Atoi(params[0])
			vValue.SetInt(int64(v))
		case reflect.Float32, reflect.Float64:
			v, _ := strconv.ParseFloat(params[0], 64)
			vValue.SetFloat(v)
		case reflect.String:
			vValue.SetString(params[0])
		case reflect.Slice:
			elemType := vValue.Type().Elem()
			slice := reflect.MakeSlice(reflect.SliceOf(elemType), len(params), len(params))
			for i, param := range params {
				switch elemType.Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					v, _ := strconv.ParseInt(param, 10, 64)
					slice.Index(i).SetInt(v)
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					v, _ := strconv.ParseUint(param, 10, 64)
					slice.Index(i).SetUint(v)
				case reflect.Float32, reflect.Float64:
					v, _ := strconv.ParseFloat(param, 64)
					slice.Index(i).SetFloat(v)
				case reflect.String:
					slice.Index(i).SetString(param)
				default:
					if structTag.Get("param") != "" {
						return fmt.Errorf("unsupported tag param:'%s'", structField.Name)
					}
				}
			}
			vValue.Set(slice)
		case reflect.Ptr:
			pValue := reflect.New(structField.Type.Elem()).Elem()
			switch pValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				v, _ := strconv.Atoi(params[0])
				vValue.Set(reflect.ValueOf(&v))
			case reflect.String:
				vValue.Set(reflect.ValueOf(&params[0]))
			case reflect.Float32, reflect.Float64:
				v, _ := strconv.ParseFloat(params[0], 64)
				vValue.Set(reflect.ValueOf(&v))
			case reflect.Bool:
				v, _ := strconv.ParseBool(params[0])
				vValue.Set(reflect.ValueOf(&v))
			default:
			}
		default:
			if structTag.Get("param") != "" {
				return fmt.Errorf("unsupported tag param:'%s'", structField.Name)
			}
		}
	}

	return
}

func SetPath(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	m := data.(map[string]string)

	var value string
	name := structField.Name
	// 首字母变小
	lName := strings.ToLower(name[:1]) + name[1:]
	for k, v := range m {
		if ToCamel(k) == lName || name == k {
			value = v
			break
		}
	}

	if value != "" {
		err = commonSet(structField, vValue, []string{value})
	}
	return
}

func CheckNil(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	if !containTag(structField.Tag, "verf") {
		return
	}

	verf := structField.Tag.Get("verf")

	switch vValue.Kind() {
	case reflect.Ptr:
		pValue := reflect.New(structField.Type.Elem()).Elem()
		switch pValue.Kind() {
		case reflect.Struct:
			for i := 0; i < pValue.NumField(); i++ {
				field := vValue.Elem().Field(i)
				fieldStruct := pValue.Type().Field(i)
				if err = CheckNil(fieldStruct, field, data); err != nil {
					return
				}
			}
		default:
			if verf != "nilable" && vValue.IsNil() {
				return fmt.Errorf("%s is missing", structField.Name)
			}
		}
	case reflect.Struct:
		t := structField.Type
		for i := 0; i < t.NumField(); i++ {
			fieldStruct := t.Field(i)
			if err = CheckNil(fieldStruct, vValue.Field(i), data); err != nil {
				return
			}
		}
	default:
		if verf != "nilable" && vValue.IsZero() {
			return fmt.Errorf("%s is missing", structField.Name)
		}
	}

	return
}

func CheckInList(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	if structField.Tag.Get("verf") == "nilable" && vValue.IsZero() {
		return
	}

	tagName := "inList"
	inList := strings.Split(structField.Tag.Get(tagName), ",")

	var values []string
	switch vValue.Kind() {
	case reflect.Ptr:
		// 获取指针指向的值
		indirectValue := vValue.Elem()

		// 递归处理指针指向的值
		switch indirectValue.Kind() {
		case reflect.Struct:
			t := indirectValue.Type()
			for i := 0; i < t.NumField(); i++ {
				fieldStruct := t.Field(i)
				if err = CheckInList(fieldStruct, indirectValue.Field(i), data); err != nil {
					return
				}
			}
			return
		default:
			return CheckInList(structField, indirectValue, data)
		}
	case reflect.Struct:
		t := structField.Type
		for i := 0; i < t.NumField(); i++ {
			fieldStruct := t.Field(i)
			if err = CheckInList(fieldStruct, vValue.Field(i), data); err != nil {
				return
			}
		}
		return
	case reflect.Slice:
		if !containTag(structField.Tag, tagName) {
			return
		}
		for i := 0; i < vValue.Len(); i++ {
			elementValue := vValue.Index(i)
			switch elementValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				values = append(values, fmt.Sprintf("%d", elementValue.Int()))
			case reflect.Float32, reflect.Float64:
				values = append(values, fmt.Sprintf("%f", elementValue.Float()))
			case reflect.String:
				values = append(values, elementValue.String())
			default:
				return
			}
		}
	case reflect.Map:
		if !containTag(structField.Tag, tagName) {
			return
		}
		keys := vValue.MapKeys()
		for _, key := range keys {
			elementValue := vValue.MapIndex(key)
			switch elementValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				values = append(values, fmt.Sprintf("%d", elementValue.Int()))
			case reflect.Float32, reflect.Float64:
				values = append(values, fmt.Sprintf("%f", elementValue.Float()))
			case reflect.String:
				values = append(values, elementValue.String())
			default:
				return
			}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if !containTag(structField.Tag, tagName) {
			return
		}
		values = append(values, fmt.Sprintf("%d", vValue.Int()))
	case reflect.Float32, reflect.Float64:
		if !containTag(structField.Tag, tagName) {
			return
		}
		values = append(values, fmt.Sprintf("%f", vValue.Float()))
	case reflect.String:
		if !containTag(structField.Tag, tagName) {
			return
		}
		values = append(values, vValue.String())
	default:
		return
	}

	if len(values) == 0 {
		return fmt.Errorf("%s is not in %v", structField.Name, strings.Join(inList, ","))
	}

	for _, value := range values {
		if !StringSliceContains(inList, value) {
			return fmt.Errorf("%s is not in %v", structField.Name, strings.Join(inList, ","))
		}
	}

	return
}

func CheckRegxp(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	if structField.Tag.Get("verf") == "nilable" && vValue.IsZero() {
		return
	}

	tagName := "reg"
	reg := structField.Tag.Get(tagName)

	var values []string
	switch vValue.Kind() {
	case reflect.Ptr:
		// 获取指针指向的值
		indirectValue := vValue.Elem()

		// 递归处理指针指向的值
		switch indirectValue.Kind() {
		case reflect.Struct:
			t := indirectValue.Type()
			for i := 0; i < t.NumField(); i++ {
				fieldStruct := t.Field(i)
				if err = CheckRegxp(fieldStruct, indirectValue.Field(i), data); err != nil {
					return
				}
			}
			return
		default:
			return CheckRegxp(structField, indirectValue, data)
		}
	case reflect.Struct:
		t := structField.Type
		for i := 0; i < t.NumField(); i++ {
			fieldStruct := t.Field(i)
			if err = CheckRegxp(fieldStruct, vValue.Field(i), data); err != nil {
				return
			}
		}
		return
	case reflect.Slice:
		if !containTag(structField.Tag, tagName) {
			return
		}
		for i := 0; i < vValue.Len(); i++ {
			elementValue := vValue.Index(i)
			switch elementValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				values = append(values, fmt.Sprintf("%d", elementValue.Int()))
			case reflect.Float32, reflect.Float64:
				values = append(values, fmt.Sprintf("%f", elementValue.Float()))
			case reflect.String:
				values = append(values, elementValue.String())
			default:
				return
			}
		}
	case reflect.Map:
		if !containTag(structField.Tag, tagName) {
			return
		}
		keys := vValue.MapKeys()
		for _, key := range keys {
			elementValue := vValue.MapIndex(key)
			switch elementValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				values = append(values, fmt.Sprintf("%d", elementValue.Int()))
			case reflect.Float32, reflect.Float64:
				values = append(values, fmt.Sprintf("%f", elementValue.Float()))
			case reflect.String:
				values = append(values, elementValue.String())
			default:
				return
			}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if !containTag(structField.Tag, tagName) {
			return
		}
		values = append(values, fmt.Sprintf("%d", vValue.Int()))
	case reflect.Float32, reflect.Float64:
		if !containTag(structField.Tag, tagName) {
			return
		}
		values = append(values, fmt.Sprintf("%f", vValue.Float()))
	case reflect.String:
		if !containTag(structField.Tag, tagName) {
			return
		}
		values = append(values, vValue.String())
	default:
		return
	}

	if len(values) == 0 {
		return fmt.Errorf("%s is not match %s", structField.Name, reg)
	}

	for _, value := range values {
		if !MatchReg(value, reg) {
			return fmt.Errorf("%s is not match %s", structField.Name, reg)
		}
	}

	return
}

func Between(structField reflect.StructField, vValue reflect.Value, data interface{}) (err error) {
	if structField.Tag.Get("verf") == "nilable" && vValue.IsZero() {
		return
	}

	tagName := "between"
	tagValue := strings.Split(structField.Tag.Get(tagName), ",")
	if len(tagValue) == 0 {
		return
	}

	var values []string
	switch vValue.Kind() {
	case reflect.Ptr:
		// 获取指针指向的值
		indirectValue := vValue.Elem()

		// 递归处理指针指向的值
		switch indirectValue.Kind() {
		case reflect.Struct:
			t := indirectValue.Type()
			for i := 0; i < t.NumField(); i++ {
				fieldStruct := t.Field(i)
				if err = Between(fieldStruct, indirectValue.Field(i), data); err != nil {
					return
				}
			}
			return
		default:
			return Between(structField, indirectValue, data)
		}
	case reflect.Struct:
		t := structField.Type
		for i := 0; i < t.NumField(); i++ {
			fieldStruct := t.Field(i)
			if err = Between(fieldStruct, vValue.Field(i), data); err != nil {
				return
			}
		}
		return
	case reflect.Slice:
		if !containTag(structField.Tag, tagName) {
			return
		}
		for i := 0; i < vValue.Len(); i++ {
			elementValue := vValue.Index(i)
			switch elementValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				values = append(values, fmt.Sprintf("%d", elementValue.Int()))
			case reflect.Float32, reflect.Float64:
				values = append(values, fmt.Sprintf("%f", elementValue.Float()))
			case reflect.String:
				values = append(values, elementValue.String())
			default:
				return
			}
		}
	case reflect.Map:
		if !containTag(structField.Tag, tagName) {
			return
		}
		keys := vValue.MapKeys()
		for _, key := range keys {
			elementValue := vValue.MapIndex(key)
			switch elementValue.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				values = append(values, fmt.Sprintf("%d", elementValue.Int()))
			case reflect.Float32, reflect.Float64:
				values = append(values, fmt.Sprintf("%f", elementValue.Float()))
			case reflect.String:
				values = append(values, elementValue.String())
			default:
				return
			}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if !containTag(structField.Tag, tagName) {
			return
		}
		values = append(values, fmt.Sprintf("%d", vValue.Int()))
	case reflect.Float32, reflect.Float64:
		if !containTag(structField.Tag, tagName) {
			return
		}
		values = append(values, fmt.Sprintf("%f", vValue.Float()))
	case reflect.String:
		if !containTag(structField.Tag, tagName) {
			return
		}
		values = append(values, vValue.String())
	default:
		return
	}

	if len(values) == 0 {
		return fmt.Errorf("%s is not between %s", structField.Name, strings.Join(tagValue, "-"))
	}

	for _, value := range values {
		if len(tagValue) == 2 && tagValue[0] != "" && tagValue[1] != "" && (value < tagValue[0] || value > tagValue[1]) {
			return fmt.Errorf("%s is not between %s", structField.Name, strings.Join(tagValue, "-"))
		} else if tagValue[0] != "" && value < tagValue[0] {
			return fmt.Errorf("%s is must >= %s", structField.Name, tagValue[0])
		} else if len(tagValue) == 2 && tagValue[1] != "" && value > tagValue[1] {
			return fmt.Errorf("%s is must <= %s", structField.Name, tagValue[1])
		}
	}

	return
}

func commonSet(structField reflect.StructField, vValue reflect.Value, values []string) (err error) {
	if len(values) <= 0 {
		return
	}
	value := values[0]

	switch vValue.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if vValue.Int() == 0 {
			v, _ := strconv.Atoi(value)
			vValue.SetInt(int64(v))
		}
	case reflect.String:
		if vValue.String() == "" {
			vValue.SetString(value)
		}
	case reflect.Float32:
		if vValue.Float() == 0 {
			v, _ := strconv.ParseFloat(value, 32)
			vValue.SetFloat(v)
		}
	case reflect.Float64:
		if vValue.Float() == 0 {
			v, _ := strconv.ParseFloat(value, 64)
			vValue.SetFloat(v)
		}
	case reflect.Slice:
		elemType := vValue.Type().Elem()
		slice := reflect.MakeSlice(reflect.SliceOf(elemType), len(values), len(values))
		for i, param := range values {
			switch elemType.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				v, _ := strconv.ParseInt(param, 10, 64)
				slice.Index(i).SetInt(v)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				v, _ := strconv.ParseUint(param, 10, 64)
				slice.Index(i).SetUint(v)
			case reflect.Float32, reflect.Float64:
				v, _ := strconv.ParseFloat(param, 64)
				slice.Index(i).SetFloat(v)
			case reflect.String:
				slice.Index(i).SetString(param)
			default:

			}
		}
		vValue.Set(slice)
	case reflect.Struct:
		t := structField.Type
		for i := 0; i < t.NumField(); i++ {
			fieldStruct := t.Field(i)
			if err = commonSet(fieldStruct, vValue.Field(i), values); err != nil {
				return
			}
		}
	case reflect.Bool:
		v, _ := strconv.ParseBool(value)
		vValue.Set(reflect.ValueOf(&v))
	case reflect.Ptr:
		pValue := reflect.New(structField.Type.Elem()).Elem()
		switch pValue.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if vValue.IsNil() {
				v, _ := strconv.Atoi(value)
				vValue.Set(reflect.ValueOf(&v))
			}
		case reflect.String:
			if vValue.IsNil() {
				v := value
				vValue.Set(reflect.ValueOf(&v))
			}
		case reflect.Float32:
			if vValue.IsNil() {
				v, _ := strconv.ParseFloat(value, 32)
				vValue.Set(reflect.ValueOf(&v))
			}
		case reflect.Float64:
			if vValue.IsNil() {
				v, _ := strconv.ParseFloat(value, 64)
				vValue.Set(reflect.ValueOf(&v))
			}
		case reflect.Bool:
			if vValue.IsNil() {
				v, _ := strconv.ParseBool(value)
				vValue.Set(reflect.ValueOf(&v))
			}
		case reflect.Ptr:
			fmt.Println("ptr ptr no support Func[SetDefaultValueIfNil]")
		case reflect.Struct:
			if vValue.Elem().IsZero() {
				return
			}

			for i := 0; i < pValue.NumField(); i++ {
				field := vValue.Elem().Field(i)
				fieldStruct := pValue.Type().Field(i)
				if err = commonSet(fieldStruct, field, values); err != nil {
					return
				}
			}
		default:
		}
	default:

	}
	return
}

func containTag(tag reflect.StructTag, tagName string) bool {
	return regexp.MustCompile(`\b` + tagName + `\b`).Match([]byte(tag))
}
