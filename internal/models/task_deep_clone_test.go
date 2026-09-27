package models

import (
	"reflect"
	"testing"
	"time"
)

// computedTaskFields are the tree links a clone deliberately leaves zero.
var computedTaskFields = map[string]bool{"Children": true, "Parent": true, "Project": true, "Level": true}

// TestCloneTaskDeepDetachesEveryField fills every stored field of a Task with a
// non-zero value by reflection, so a field added later is covered without
// editing this test, and then proves the clone equals the source while sharing
// no pointer, slice, or map with it.
func TestCloneTaskDeepDetachesEveryField(t *testing.T) {
	var source Task
	value := reflect.ValueOf(&source).Elem()
	for i := 0; i < value.NumField(); i++ {
		field := value.Type().Field(i)
		if computedTaskFields[field.Name] {
			continue
		}
		fillNonZero(t, field.Name, value.Field(i))
	}

	clone := CloneTaskDeep(&source)
	cloned := reflect.ValueOf(clone).Elem()
	for i := 0; i < value.NumField(); i++ {
		field := value.Type().Field(i)
		if computedTaskFields[field.Name] {
			if !cloned.Field(i).IsZero() {
				t.Errorf("%s = %v, want the computed link left zero", field.Name, cloned.Field(i))
			}
			continue
		}
		if !reflect.DeepEqual(value.Field(i).Interface(), cloned.Field(i).Interface()) {
			t.Errorf("%s = %#v, want %#v", field.Name, cloned.Field(i).Interface(), value.Field(i).Interface())
		}
		if path, shared := sharesMemory(value.Field(i), cloned.Field(i), field.Name); shared {
			t.Errorf("%s shares memory with the source; CloneTaskDeep must copy it", path)
		}
	}
}

func TestCloneTaskDeepKeepsTimeLocations(t *testing.T) {
	zone := time.FixedZone("UTC-4", -4*60*60)
	due := time.Date(2026, time.October, 5, 0, 0, 0, 0, zone)
	clone := CloneTaskDeep(&Task{DueDate: &due})
	if clone.DueDate.Location() != zone {
		t.Errorf("clone location = %v, want the source's %v", clone.DueDate.Location(), zone)
	}
}

func TestCloneTaskDeepClonesEmptyCollectionsAsNil(t *testing.T) {
	clone := CloneTaskDeep(&Task{Tags: []string{}, Checklist: []ChecklistItem{}, Dependencies: []TaskReference{}})
	if clone.Tags != nil || clone.Checklist != nil || clone.Dependencies != nil {
		t.Errorf("empty collections cloned as %#v, %#v, %#v; want nil", clone.Tags, clone.Checklist, clone.Dependencies)
	}
	if CloneTaskDeep(nil) != nil {
		t.Error("a nil task cloned to non-nil")
	}
}

func fillNonZero(t *testing.T, path string, v reflect.Value) {
	t.Helper()
	switch v.Kind() {
	case reflect.String:
		v.SetString("x")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(3)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(3)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(3)
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		fillNonZero(t, path, v.Elem())
	case reflect.Slice:
		slice := reflect.MakeSlice(v.Type(), 2, 2)
		for i := 0; i < 2; i++ {
			fillNonZero(t, path+"[]", slice.Index(i))
		}
		v.Set(slice)
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		key := reflect.New(v.Type().Key()).Elem()
		fillNonZero(t, path+"{key}", key)
		elem := reflect.New(v.Type().Elem()).Elem()
		fillNonZero(t, path+"{}", elem)
		m.SetMapIndex(key, elem)
		v.Set(m)
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(time.Time{}) {
			v.Set(reflect.ValueOf(time.Date(2026, time.September, 26, 10, 0, 0, 0, time.UTC)))
			return
		}
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				fillNonZero(t, path+"."+v.Type().Field(i).Name, v.Field(i))
			}
		}
	default:
		t.Fatalf("%s: no filler for kind %s; extend fillNonZero", path, v.Kind())
	}
}

// sharesMemory reports the first path at which the two values hold the same
// pointer, slice backing array, or map.
func sharesMemory(a, b reflect.Value, path string) (string, bool) {
	switch a.Kind() {
	case reflect.Pointer:
		if a.IsNil() || b.IsNil() {
			return "", false
		}
		if a.Pointer() == b.Pointer() {
			return path, true
		}
		return sharesMemory(a.Elem(), b.Elem(), path)
	case reflect.Slice:
		if a.Len() == 0 || b.Len() == 0 {
			return "", false
		}
		if a.Pointer() == b.Pointer() {
			return path, true
		}
		for i := 0; i < a.Len() && i < b.Len(); i++ {
			if p, shared := sharesMemory(a.Index(i), b.Index(i), path+"[]"); shared {
				return p, true
			}
		}
	case reflect.Map:
		if !a.IsNil() && !b.IsNil() && a.Pointer() == b.Pointer() {
			return path, true
		}
	case reflect.Struct:
		if a.Type() == reflect.TypeOf(time.Time{}) {
			return "", false
		}
		for i := 0; i < a.NumField(); i++ {
			if a.Type().Field(i).IsExported() {
				if p, shared := sharesMemory(a.Field(i), b.Field(i), path+"."+a.Type().Field(i).Name); shared {
					return p, true
				}
			}
		}
	default:
		// Scalars are copied by value; fillNonZero refuses any other kind, so
		// no reference-holding kind reaches this case unfilled.
	}
	return "", false
}
