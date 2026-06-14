package color

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

type Cube struct {
	Title string       `json:"title,omitempty"`
	Size  int          `json:"size"`
	Rows  [][3]float64 `json:"rows"`
}

func ParseCube(raw []byte) (Cube, error) {
	var cube Cube
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		switch fields[0] {
		case "TITLE":
			cube.Title = strings.Trim(strings.TrimPrefix(line, "TITLE"), " \"")
		case "LUT_3D_SIZE":
			if len(fields) != 2 {
				return Cube{}, fmt.Errorf("invalid LUT_3D_SIZE")
			}
			size, err := strconv.Atoi(fields[1])
			if err != nil {
				return Cube{}, err
			}
			cube.Size = size
		case "DOMAIN_MIN", "DOMAIN_MAX":
			continue
		default:
			if len(fields) != 3 {
				return Cube{}, fmt.Errorf("malformed LUT row %q", line)
			}
			var row [3]float64
			for i := range fields {
				value, err := strconv.ParseFloat(fields[i], 64)
				if err != nil {
					return Cube{}, err
				}
				row[i] = value
			}
			cube.Rows = append(cube.Rows, row)
		}
	}
	if cube.Size <= 0 {
		return Cube{}, fmt.Errorf("missing LUT_3D_SIZE")
	}
	expected := cube.Size * cube.Size * cube.Size
	if len(cube.Rows) != expected {
		return Cube{}, fmt.Errorf("expected %d LUT rows, got %d", expected, len(cube.Rows))
	}
	return cube, scanner.Err()
}
