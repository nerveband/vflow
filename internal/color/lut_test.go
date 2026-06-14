package color

import "testing"

func TestParseCubeLUT(t *testing.T) {
	raw := []byte("TITLE \"basic\"\nLUT_3D_SIZE 2\n0 0 0\n0 0 1\n0 1 0\n0 1 1\n1 0 0\n1 0 1\n1 1 0\n1 1 1\n")
	lut, err := ParseCube(raw)
	if err != nil {
		t.Fatal(err)
	}
	if lut.Title != "basic" || lut.Size != 2 || len(lut.Rows) != 8 {
		t.Fatalf("unexpected LUT: %#v", lut)
	}
}
