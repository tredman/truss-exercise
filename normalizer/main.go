package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	pacificLoc, _ = time.LoadLocation("US/Pacific")
	easternLoc, _ = time.LoadLocation("US/Eastern")
)

// The csv lib parses for us just fine, but it gives us back []string slices
// that are tedious to work with. We'll marshal these into a data structure instead
type Record struct {
	Timestamp     string
	Address       string
	Zip           string
	FullName      string
	FooDuration   string
	BarDuration   string
	TotalDuration string
	Notes         string
}

func validateUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}

	// This is new in go 1.13 as a convenience. Were it not there I would
	// convert the string to []rune and walk it, checking each rune with
	// utf8.ValidRune() and replacing failed runes with RuneError
	return strings.ToValidUTF8(s, string(utf8.RuneError))
}

func newRecord(fields []string) *Record {
	for i := range fields {
		fields[i] = validateUTF8(fields[i])
	}
	return &Record{
		Timestamp:     fields[0],
		Address:       fields[1],
		Zip:           fields[2],
		FullName:      fields[3],
		FooDuration:   fields[4],
		BarDuration:   fields[5],
		TotalDuration: fields[6],
		Notes:         fields[7],
	}
}

// Normalize does our laundry list of changes to the input record in-place
// If it fails we'll have a partially normalized record that should be skipped
func (r *Record) Normalize() error {
	// Examining the sample it looks like there's only one time format to deal with
	// Parse as though in US/Pacific time
	t, err := time.ParseInLocation("1/2/06 3:04:05 PM", r.Timestamp, pacificLoc)
	if err != nil {
		return err
	}
	// Convert to Eastern Time before rendering as RFC3339
	r.Timestamp = t.In(easternLoc).Format(time.RFC3339)

	// Go AFAICT doesn't have a good way to handle durations expressed as
	// HH:MM:SS.MS so we'll just parse this ourselves

	var hour, minute, second, msec time.Duration
	scanned, _ := fmt.Sscanf(r.FooDuration, "%d:%d:%d.%d", &hour, &minute, &second, &msec)
	if scanned != 4 {
		return fmt.Errorf("bad format for FooDuration")
	}
	fooDuration := (time.Hour * hour) + (time.Minute * minute) + (time.Second * second) + (time.Millisecond * msec)

	scanned, _ = fmt.Sscanf(r.BarDuration, "%d:%d:%d.%d", &hour, &minute, &second, &msec)
	if scanned != 4 {
		return fmt.Errorf("bad format for BarDuration")
	}
	barDuration := (time.Hour * hour) + (time.Minute * minute) + (time.Second * second) + (time.Millisecond * msec)

	totalDuration := fooDuration + barDuration

	r.FooDuration = fmt.Sprintf("%f", fooDuration.Seconds())
	r.BarDuration = fmt.Sprintf("%f", barDuration.Seconds())
	r.TotalDuration = fmt.Sprintf("%f", totalDuration.Seconds())

	// Pad zips shorter than 5 digits with zeroes on the left
	// Seems weird to pad a string type with zeroes (as opposed to a int type)
	// but it works for this case
	r.Zip = fmt.Sprintf("%05s", r.Zip)

	// Full name is converted to uppercase
	r.FullName = strings.ToUpper(r.FullName)
	return nil
}

// Returns a []string that can be fed to a CSV Writer
func (r *Record) Fields() []string {
	return []string{
		r.Timestamp,
		r.Address,
		r.Zip,
		r.FullName,
		r.FooDuration,
		r.BarDuration,
		r.TotalDuration,
		r.Notes,
	}
}

func main() {
	// I'm using Go's CSV package, which is part of its standard library.
	reader := csv.NewReader(os.Stdin)
	// Unless I missed it, we expect the number of fields to be consistent
	// for each row. This will cause an error if the field count is wrong.
	reader.FieldsPerRecord = 8

	writer := csv.NewWriter(os.Stdout)
	defer writer.Flush()

	// Consume the first line, which contains the headers. We can feed these
	// to the writer when outputting our normalized CSV
	headers, err := reader.Read()
	if err != nil {
		fmt.Fprintln(os.Stderr, "unexpected error reading csv header: ", err.Error())
	}
	writer.Write(headers)

	fields, err := reader.Read()
	for err == nil {
		// Skip totally empty lines
		if fields != nil {
			record := newRecord(fields)

			// Debug output, can remove
			// fmt.Printf("%+v\n", record)

			err := record.Normalize()
			if err != nil {
				line := strings.Join(fields, ",") // rebuild the line so we can render the one with the error
				fmt.Fprintln(os.Stderr, "normalization error: ", err.Error(), " for line \"", line, "\"")
			}

			err = writer.Write(record.Fields())
			if err != nil {
				fmt.Fprintln(os.Stderr, "unexpected error writing fields: ", err.Error())
			}

			// Debug output, can remove
			// fmt.Printf("%+v\n", record)
		}

		fields, err = reader.Read()
	}
	// reader returns io.EOF if everything went well
	if err != nil && err != io.EOF {
		fmt.Fprintln(os.Stderr, "unexpected error: ", err.Error())
	}
}
