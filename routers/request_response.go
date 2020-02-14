package routers

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
)

// PageRequest normal page request
type PageRequest struct {
	Start int    `form:"start"`
	Limit int    `form:"limit"`
	Q     string `form:"q"`
	Sort  string `form:"sort"`
	Order string `form:"order"`
}

// NewPageRequest returns
func NewPageRequest() *PageRequest {
	return &PageRequest{
		Start: 0,
		Limit: math.MaxInt32,
	}
}

// PageResponse normal page response
type PageResponse struct {
	Total int         `json:"total"`
	Rows  interface{} `json:"rows"`
}

// NewPageResponse returns
func NewPageResponse(rows interface{}) *PageResponse {
	v := reflect.ValueOf(rows)
	if v.Kind() == reflect.Slice {
		return &PageResponse{
			Total: v.Len(),
			Rows:  rows,
		}
	} else {
		return &PageResponse{
			Total: 1,
			Rows:  []interface{}{rows},
		}
	}
}

func (pr *PageResponse) Slice(start, limit int) *PageResponse {
	if limit < 0 || start < 0 {
		return pr
	}
	if pr.Rows == nil {
		return pr
	}
	v := reflect.ValueOf(pr.Rows)
	if v.Kind() != reflect.Slice {
		return pr
	}
	_start := start
	if _start > pr.Total {
		_start = pr.Total
	}
	_end := start + limit
	if _end > pr.Total {
		_end = pr.Total
	}
	size := _end - _start
	_rows := make([]interface{}, size)
	for i := 0; i < size; i++ {
		_rows[i] = v.Index(_start + i).Interface()
	}
	pr.Rows = _rows
	return pr
}

func (pr *PageResponse) Sort(by, order string) *PageResponse {
	if by == "" {
		return pr
	}
	if reflect.TypeOf(pr.Rows).Kind() != reflect.Slice {
		return pr
	}
	if reflect.ValueOf(pr.Rows).Len() == 0 {
		return pr
	}
	te := reflect.TypeOf(pr.Rows).Elem()
	for te.Kind() == reflect.Array || te.Kind() == reflect.Chan || te.Kind() == reflect.Map || te.Kind() == reflect.Ptr || te.Kind() == reflect.Slice {
		te = te.Elem()
	}
	if te.Kind() == reflect.Interface {
		va := reflect.ValueOf(pr.Rows).Index(0)
		for va.Kind() == reflect.Interface || va.Kind() == reflect.Ptr {
			va = va.Elem()
		}
		te = va.Type()
	}
	byIdx := -1
	if te.Kind() == reflect.Struct {
		for i := 0; i < te.NumField(); i++ {
			if strings.EqualFold(te.Field(i).Name, by) {
				// log.Printf("%v field name[%s] find field[%s] index[%d], case insensitive", te, by, te.Field(i).Name, i)
				byIdx = i
				break
			}
		}
		if byIdx == -1 {
			log.Printf("%v field name[%s] not found, case insensitive", te, by)
			return pr
		}
	}
	sort.Slice(pr.Rows, func(i, j int) (ret bool) {
		va := reflect.ValueOf(pr.Rows).Index(i)
		vb := reflect.ValueOf(pr.Rows).Index(j)
		for va.Kind() == reflect.Interface || va.Kind() == reflect.Ptr {
			va = va.Elem()
		}
		for vb.Kind() == reflect.Interface || vb.Kind() == reflect.Ptr {
			vb = vb.Elem()
		}
		if va.Kind() == reflect.Struct && vb.Kind() == reflect.Struct {
			switch va.Field(byIdx).Kind() {
			case reflect.Float32, reflect.Float64:
				ret = va.Field(byIdx).Float() < vb.Field(byIdx).Float()
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				ret = va.Field(byIdx).Int() < vb.Field(byIdx).Int()
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				ret = va.Field(byIdx).Uint() < vb.Field(byIdx).Uint()
			default:
				ret = fmt.Sprintf("%v", va.Field(byIdx)) < fmt.Sprintf("%v", vb.Field(byIdx))
			}
		} else if va.Kind() == reflect.Map && vb.Kind() == reflect.Map {
			ret = fmt.Sprintf("%v", va.MapIndex(reflect.ValueOf(by))) < fmt.Sprintf("%v", vb.MapIndex(reflect.ValueOf(by)))
		}
		if strings.HasPrefix(strings.ToLower(order), "desc") {
			ret = !ret
		}
		return
	})
	return pr
}
