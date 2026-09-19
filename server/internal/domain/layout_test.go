package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func validTable() TableFields {
	return TableFields{Label: "15", Shape: ShapeRect, X: 20, Y: 30, W: 8, H: 6, Rotation: 0, Seats: 4}
}

func TestValidateTable(t *testing.T) {
	got, err := ValidateTable(TableFields{Label: " 15 ", Shape: ShapeCircle, X: 0, Y: 0, W: 40, H: 40, Seats: 30, Rotation: 359}, "")
	require.NoError(t, err)
	require.Equal(t, "15", got.Label)

	cases := []struct {
		name   string
		mutate func(*TableFields)
		field  string
		reason string
	}{
		{"table sticking out on the right is out of bounds", func(f *TableFields) { f.X, f.W = 95, 10 }, "x", ReasonOutOfBounds},
		{"table touching the right edge fits", func(f *TableFields) { f.X, f.W = 90, 10 }, "", ""},
		{"table sticking out at the bottom is out of bounds", func(f *TableFields) { f.Y, f.H = 70, 31 }, "y", ReasonOutOfBounds},
		{"negative x is invalid", func(f *TableFields) { f.X = -1 }, "x", ReasonInvalid},
		{"zero width is invalid", func(f *TableFields) { f.W = 0 }, "w", ReasonInvalid},
		{"width over 40 is invalid", func(f *TableFields) { f.W = 41 }, "w", ReasonInvalid},
		{"rotation of 360 is invalid", func(f *TableFields) { f.Rotation = 360 }, "rotation", ReasonInvalid},
		{"31 seats are invalid", func(f *TableFields) { f.Seats = 31 }, "seats", ReasonInvalid},
		{"empty label is required", func(f *TableFields) { f.Label = "  " }, "label", ReasonRequired},
		{"nine character label is too long", func(f *TableFields) { f.Label = "123456789" }, "label", ReasonTooLong},
		{"triangle shape is invalid", func(f *TableFields) { f.Shape = "triangle" }, "shape", ReasonInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := validTable()
			c.mutate(&f)
			_, err := ValidateTable(f, "")
			if c.field == "" {
				require.NoError(t, err)
				return
			}
			var v *ValidationError
			require.ErrorAs(t, err, &v)
			require.Equal(t, c.reason, v.Fields[c.field])
		})
	}
}

func TestValidTablesStayInsideTheGrid(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		f := TableFields{
			Label: "1", Shape: ShapeRect,
			X: rapid.IntRange(-5, 105).Draw(t, "x"), Y: rapid.IntRange(-5, 105).Draw(t, "y"),
			W: rapid.IntRange(0, 45).Draw(t, "w"), H: rapid.IntRange(0, 45).Draw(t, "h"),
			Seats: 4,
		}
		if _, err := ValidateTable(f, ""); err == nil {
			if f.X < 0 || f.Y < 0 || f.X+f.W > Grid || f.Y+f.H > Grid || f.W < 1 || f.H < 1 {
				t.Fatalf("accepted a table outside the grid: %+v", f)
			}
		}
	})
}

func TestTablePatchKeepsUntouchedFields(t *testing.T) {
	x, label := 50, "22"
	got := TablePatch{X: &x, Label: &label}.Apply(validTable())
	want := validTable()
	want.X, want.Label = 50, "22"
	require.Equal(t, want, got)
}

func TestValidateLayout(t *testing.T) {
	current := map[string]Table{
		"a": {ID: "a", Label: "1", Shape: ShapeRect, X: 0, Y: 0, W: 10, H: 10, Seats: 4},
		"b": {ID: "b", Label: "2", Shape: ShapeRect, X: 20, Y: 0, W: 10, H: 10, Seats: 4},
	}
	require.NoError(t, ValidateLayout([]Placement{{ID: "a", X: 5, Y: 5, W: 10, H: 10}}, current))

	var v *ValidationError
	require.ErrorAs(t, ValidateLayout(nil, current), &v)
	require.Equal(t, ReasonRequired, v.Fields["tables"])

	require.ErrorAs(t, ValidateLayout([]Placement{{ID: "a", W: 10, H: 10}, {ID: "a", W: 10, H: 10}}, current), &v)
	require.Equal(t, ReasonInvalid, v.Fields["tables"])

	require.ErrorAs(t, ValidateLayout([]Placement{{ID: "a", W: 10, H: 10}, {ID: "b", X: 95, W: 10, H: 10}}, current), &v)
	require.Equal(t, ReasonOutOfBounds, v.Fields["tables[1].x"])
}

func TestValidateHall(t *testing.T) {
	name, err := ValidateHall("  Веранда ", 2)
	require.NoError(t, err)
	require.Equal(t, "Веранда", name)
	var v *ValidationError
	_, err = ValidateHall("", -1)
	require.ErrorAs(t, err, &v)
	require.Equal(t, map[string]string{"name": ReasonRequired, "sort_order": ReasonInvalid}, v.Fields)
}
