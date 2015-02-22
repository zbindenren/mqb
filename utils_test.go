package mqb

import "testing"

type TestAStructName struct{}

func TestStructName(t *testing.T) {
	if structName(TestAStructName{}) != "testastructname" {
		t.Errorf("wrong structname generated")
	}

	if structName(&TestAStructName{}) != "testastructname" {
		t.Errorf("wrong structname generated")
	}
}
