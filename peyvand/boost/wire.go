package boost

import (
	"encoding/json"
	"errors"
	"io"
)

const maxLine = 4096

// readLine reads one '\n'-terminated line one byte at a time, so nothing after it is consumed
// from the connection (the raw relay starts right after the last line).
func readLine(r io.Reader) ([]byte, error) {
	var buf []byte
	b := make([]byte, 1)
	for {
		if _, err := io.ReadFull(r, b); err != nil {
			return nil, err
		}
		if b[0] == '\n' {
			return buf, nil
		}
		buf = append(buf, b[0])
		if len(buf) > maxLine {
			return nil, errors.New("boost: line too long")
		}
	}
}

func readJSON(r io.Reader, v any) error {
	line, err := readLine(r)
	if err != nil {
		return err
	}
	return json.Unmarshal(line, v)
}

func writeJSON(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}
