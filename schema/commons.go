package schema

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Custom sql type.
// A custom sql type T must implements Valuer and *T must implements Scanner
type Tags []string

var _ sql.Scanner = (*Tags)(nil)
var _ driver.Valuer = (Tags)(nil)

func (t Tags) GetMeta(name string) string {
	for _, tag := range t {
		if strings.HasPrefix(tag, name+":") {
			return tag[len(name)+1:]
		}
	}
	return ""
}

func (t Tags) GetMetaArray(name string) (arr []string) {
	for _, tag := range t {
		if strings.HasPrefix(tag, name+":") {
			arr = append(arr, tag[len(name)+1:])
		}
	}
	return
}

func (t *Tags) Scan(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal value:", value))
	}
	err := json.Unmarshal([]byte(str), &t)
	return err
}

func (t Tags) Value() (driver.Value, error) {
	if len(t) == 0 {
		return "", nil
	}
	bytes, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
