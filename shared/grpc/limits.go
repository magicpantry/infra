package grpc

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

func EnforceLimits(x any, maxListLength, maxStringLength int) error {
	return enforceLimits("", x, maxListLength, maxStringLength)
}

func enforceLimits(name string, x any, maxListLength, maxStringLength int) error {
	v := reflect.ValueOf(x)

	if !v.IsValid() {
		return nil
	}
	if v.Kind() == reflect.String {
		if len(v.String()) > maxStringLength {
			return fmt.Errorf(
				"string '%s' length %s is longer than limit %s",
				toSnakeCase(name), len(v.String()), maxStringLength)
		}
	}
	if v.Kind() == reflect.Slice {
		if v.Len() > maxListLength {
			return fmt.Errorf(
				"list '%s' length %d is longer than limit %d",
				toSnakeCase(name), v.Len(), maxListLength)
		}
	}

	if v.Kind() == reflect.Pointer {
		if v.Elem().IsValid() && v.Elem().CanInterface() {
			return enforceLimits(name, v.Elem().Interface(), maxListLength, maxStringLength)
		}
	}
	if v.Kind() == reflect.Struct {
		for i := 0; i < v.NumField(); i++ {
			if !v.Field(i).CanInterface() {
				continue
			}
			if err := enforceLimits(
				joinDot(name, v.Type().Field(i).Name),
				v.Field(i).Interface(), maxListLength, maxStringLength); err != nil {
				return err
			}
		}
	}
	if v.Kind() == reflect.Slice {
		for i := 0; i < v.Len(); i++ {
			if !v.Index(i).CanInterface() {
				continue
			}
			if err := enforceLimits(
				fmt.Sprintf("%s[%d]", name, i),
				v.Index(i).Interface(), maxListLength, maxStringLength); err != nil {
				return err
			}
		}
	}

	return nil
}

func joinDot(a, b string) string {
	if a == "" {
		return b
	}
	return a + "." + b
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func toSnakeCase(v string) string {
	snake := matchFirstCap.ReplaceAllString(v, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(v, "${1}_${2}")
	return strings.ReplaceAll(strings.ToLower(snake), "._", ".")
}
