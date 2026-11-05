package api

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestExport(t *testing.T) {
	h, c := newTestServer(t)
	mustCall(t, h, c, "POST", "/api/fields", `{"name":"Voltage","type":"text"}`, 201)
	box := add(t, h, c, fmt.Sprintf(`{"name":"Box","kindId":%d}`, kindID(t, h, c, "box")))
	drill := add(t, h, c, fmt.Sprintf(`{"name":"Vrtačka, \"Makita\"","parentId":%d,"purchasePrice":99.5,"tags":["Tools","Power"],"fields":{"1":"18 V"}}`, box.ID))
	upload(t, h, c, fmt.Sprintf("/api/things/%d/photos", drill.ID), map[string][]byte{"a.png": pngBytes(t)})
	upload(t, h, c, fmt.Sprintf("/api/things/%d/attachments", drill.ID), map[string][]byte{"manual.pdf": []byte("%PDF-1.4")})
	add(t, h, c, `{"name":"Gone","soldDate":"2026-01-01"}`)

	rec := mustCall(t, h, c, "GET", "/api/export.csv", "", 200)
	body := strings.TrimPrefix(rec.Body.String(), "\ufeff")
	rows, err := csv.NewReader(strings.NewReader(body)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 || rows[0][len(rows[0])-1] != "Voltage" {
		t.Fatalf("csv = %v", rows)
	}
	var drillRow []string
	for _, r := range rows {
		if strings.HasPrefix(r[3], "Vrtačka") {
			drillRow = r
		}
	}
	if drillRow == nil || drillRow[2] != "Box" || drillRow[12] != "99.5" || drillRow[len(drillRow)-1] != "18 V" {
		t.Errorf("drill row = %v", drillRow)
	}

	rec = mustCall(t, h, c, "GET", "/api/export.zip", "", 200)
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]*zip.File{}
	for _, f := range zr.File {
		names[strings.SplitN(f.Name, "/", 2)[0]] = f
	}
	if names["wherehouse.json"] == nil || names["photos"] == nil || names["documents"] == nil {
		t.Fatalf("archive holds %v", names)
	}
	rc, _ := names["wherehouse.json"].Open()
	data, _ := io.ReadAll(rc)
	var b backup
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatal(err)
	}
	if len(b.Things) != 3 || len(b.Kinds) != 5 || len(b.Photos) != 1 || len(b.Attachments) != 1 {
		t.Errorf("backup = %d things, %d kinds, %v, %v", len(b.Things), len(b.Kinds), b.Photos, b.Attachments)
	}
}
