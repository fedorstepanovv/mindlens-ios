package metrics

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Records is what Read found: the two kinds, separated.
type Records struct {
	Lanes    []LaneRecord
	Outcomes []OutcomeRecord
}

// Read loads every record under the paths given. A path may be a .jsonl file or a
// directory, which is walked for .jsonl files — the shape a downloaded artifact has.
// One bad line fails the read and names itself: a report that silently dropped
// records would be a report about a different gate.
func Read(paths ...string) (Records, error) {
	var recs Records
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return recs, err
		}
		if !info.IsDir() {
			if err := readFile(p, &recs); err != nil {
				return recs, err
			}
			continue
		}
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
				return err
			}
			return readFile(path, &recs)
		})
		if err != nil {
			return recs, err
		}
	}
	return recs, nil
}

func readFile(path string, recs *Records) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var head struct {
			Kind   Kind `json:"kind"`
			Schema int  `json:"schema"`
		}
		if err := json.Unmarshal(line, &head); err != nil {
			return fmt.Errorf("%s:%d: not a record: %w", path, n, err)
		}
		if head.Schema > SchemaVersion {
			return fmt.Errorf("%s:%d: record is schema %d, this reader knows %d", path, n, head.Schema, SchemaVersion)
		}
		switch head.Kind {
		case KindLane:
			var r LaneRecord
			if err := json.Unmarshal(line, &r); err != nil {
				return fmt.Errorf("%s:%d: %w", path, n, err)
			}
			recs.Lanes = append(recs.Lanes, r)
		case KindOutcome:
			var r OutcomeRecord
			if err := json.Unmarshal(line, &r); err != nil {
				return fmt.Errorf("%s:%d: %w", path, n, err)
			}
			recs.Outcomes = append(recs.Outcomes, r)
		default:
			return fmt.Errorf("%s:%d: record kind %q is not lane or outcome", path, n, head.Kind)
		}
	}
	return sc.Err()
}

// Append writes records to a JSONL file, one line each, creating it if needed.
func Append(path string, records ...any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, r := range records {
		line, err := json.Marshal(r)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(line, '\n')); err != nil {
			return err
		}
	}
	return nil
}
