package workspace

import (
	"reflect"
	"testing"
)

func TestDefaultInfisicalEnvironments(t *testing.T) {
	if !reflect.DeepEqual(DefaultEnvironments, []string{"dev", "staging", "prod"}) {
		t.Fatal(DefaultEnvironments)
	}
}
