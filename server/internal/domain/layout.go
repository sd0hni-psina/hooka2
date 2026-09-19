package domain

import (
	"strconv"
	"strings"
)

// Grid is the hall size in layout units, tech.md section 7.
const Grid = 100

// TableFields are the editable fields of a table.
type TableFields struct {
	Label    string
	Shape    Shape
	X, Y     int
	W, H     int
	Rotation int
	Seats    int
}

func (t Table) Fields() TableFields {
	return TableFields{Label: t.Label, Shape: t.Shape, X: t.X, Y: t.Y, W: t.W, H: t.H, Rotation: t.Rotation, Seats: t.Seats}
}

// ValidateTable checks one table against tech.md section 19.6. prefix names
// the table inside a batch, e.g. "tables[3].", and is empty otherwise.
func ValidateTable(t TableFields, prefix string) (TableFields, error) {
	t.Label = strings.TrimSpace(t.Label)
	var v ValidationError
	CheckTable(&v, t, prefix)
	return t, v.Err()
}

// CheckTable adds the problems of one table to v.
func CheckTable(v *ValidationError, t TableFields, prefix string) {
	checkLen(v, prefix+"label", strings.TrimSpace(t.Label), 1, 8)
	if !t.Shape.Valid() {
		v.Add(prefix+"shape", ReasonInvalid)
	}
	inRange := func(field string, val, lo, hi int) bool {
		if val < lo || val > hi {
			v.Add(prefix+field, ReasonInvalid)
			return false
		}
		return true
	}
	okX := inRange("x", t.X, 0, Grid-1)
	okY := inRange("y", t.Y, 0, Grid-1)
	okW := inRange("w", t.W, 1, 40)
	okH := inRange("h", t.H, 1, 40)
	inRange("rotation", t.Rotation, 0, 359)
	inRange("seats", t.Seats, 1, 30)
	if okX && okW && t.X+t.W > Grid {
		v.Add(prefix+"x", ReasonOutOfBounds)
	}
	if okY && okH && t.Y+t.H > Grid {
		v.Add(prefix+"y", ReasonOutOfBounds)
	}
}

// TablePatch is a partial update; nil fields keep their value.
type TablePatch struct {
	Label    *string
	Shape    *Shape
	X, Y     *int
	W, H     *int
	Rotation *int
	Seats    *int
	Archived *bool
}

// Apply merges the patch into the current fields.
func (p TablePatch) Apply(t TableFields) TableFields {
	set := func(dst *int, src *int) {
		if src != nil {
			*dst = *src
		}
	}
	if p.Label != nil {
		t.Label = *p.Label
	}
	if p.Shape != nil {
		t.Shape = *p.Shape
	}
	set(&t.X, p.X)
	set(&t.Y, p.Y)
	set(&t.W, p.W)
	set(&t.H, p.H)
	set(&t.Rotation, p.Rotation)
	set(&t.Seats, p.Seats)
	return t
}

// ValidateHall checks a hall name and sort order.
func ValidateHall(name string, sortOrder int) (string, error) {
	name = strings.TrimSpace(name)
	var v ValidationError
	checkLen(&v, "name", name, 1, 60)
	if sortOrder < 0 || sortOrder > 1000 {
		v.Add("sort_order", ReasonInvalid)
	}
	return name, v.Err()
}

// Placement is the geometry part of a batch layout update.
type Placement struct {
	ID       string
	X, Y     int
	W, H     int
	Rotation int
}

// ValidateLayout checks a batch: no duplicate ids, every table fits. current
// maps id to the stored table so label, shape and seats stay as they are.
func ValidateLayout(batch []Placement, current map[string]Table) error {
	var v ValidationError
	if len(batch) == 0 {
		v.Add("tables", ReasonRequired)
		return v.Err()
	}
	seen := map[string]bool{}
	for i, p := range batch {
		if seen[p.ID] {
			v.Add("tables", ReasonInvalid)
			continue
		}
		seen[p.ID] = true
		cur, ok := current[p.ID]
		if !ok {
			continue
		}
		f := cur.Fields()
		f.X, f.Y, f.W, f.H, f.Rotation = p.X, p.Y, p.W, p.H, p.Rotation
		CheckTable(&v, f, "tables["+strconv.Itoa(i)+"].")
	}
	return v.Err()
}
