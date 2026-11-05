package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/fhavrlent/wherehouse/internal/store"
)

func TestTagsAndFields(t *testing.T) {
	h, c := newTestServer(t)
	voltage := decodeAs[store.Field](t, mustCall(t, h, c, "POST", "/api/fields", `{"name":"Voltage","type":"number"}`, 201))
	manual := decodeAs[store.Field](t, mustCall(t, h, c, "POST", "/api/fields", `{"name":"Manual online","type":"url"}`, 201))
	mustCall(t, h, c, "POST", "/api/fields", `{"name":"voltage","type":"text"}`, 400)
	mustCall(t, h, c, "POST", "/api/fields", `{"name":"X","type":"color"}`, 400)

	it := add(t, h, c, fmt.Sprintf(`{"name":"Drill","tags":["Tools","#work"," tools "],"fields":{"%d":"18,5","%d":"https://example.com/m"}}`,
		voltage.ID, manual.ID))
	if len(it.Tags) != 2 || len(it.Fields) != 2 || it.Fields[0].Value != "18.5" {
		t.Fatalf("created = %+v", it)
	}
	mustCall(t, h, c, "POST", "/api/things", fmt.Sprintf(`{"name":"Bad","fields":{"%d":"ftp://x"}}`, manual.ID), 400)
	for _, q := range []string{"work", "18.5"} {
		if res := decodeAs[[]store.Thing](t, mustCall(t, h, c, "GET", "/api/search?q="+q, "", 200)); len(res) != 1 {
			t.Errorf("search %q = %+v", q, res)
		}
	}
	tags := decodeAs[[]store.Tag](t, mustCall(t, h, c, "GET", "/api/tags", "", 200))
	if len(tags) != 2 || tags[0].Count != 1 {
		t.Fatalf("tags = %+v", tags)
	}
	mustCall(t, h, c, "PUT", fmt.Sprintf("/api/things/%d", it.ID), `{"name":"Drill"}`, 200)
	if got := decodeAs[thingResponse](t, mustCall(t, h, c, "GET", fmt.Sprintf("/api/things/%d", it.ID), "", 200)); len(got.Tags) != 2 {
		t.Errorf("tags after edit = %+v", got.Tags)
	}
	work := tags[1]
	mustCall(t, h, c, "PATCH", fmt.Sprintf("/api/tags/%d", work.ID), `{"name":"Office","color":"blue"}`, 204)
	if tagged := decodeAs[tagResponse](t, mustCall(t, h, c, "GET", fmt.Sprintf("/api/tags/%d", work.ID), "", 200)); tagged.Name != "Office" || len(tagged.Things) != 1 {
		t.Errorf("tag = %+v", tagged)
	}
	mustCall(t, h, c, "PUT", fmt.Sprintf("/api/things/%d", it.ID), `{"name":"Drill","tags":[]}`, 200)
	if tags := decodeAs[[]store.Tag](t, mustCall(t, h, c, "GET", "/api/tags", "", 200)); len(tags) != 0 {
		t.Errorf("tags left = %+v", tags)
	}
	mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/fields/%d", voltage.ID), "", 204)
	if got := decodeAs[thingResponse](t, mustCall(t, h, c, "GET", fmt.Sprintf("/api/things/%d", it.ID), "", 200)); len(got.Fields) != 1 {
		t.Errorf("fields after delete = %+v", got.Fields)
	}
}

func TestAttachmentsAndMaintenance(t *testing.T) {
	h, c := newTestServer(t)
	it := add(t, h, c, `{"name":"Boiler"}`)
	rec := upload(t, h, c, fmt.Sprintf("/api/things/%d/attachments", it.ID), map[string][]byte{"manual.pdf": []byte("%PDF-1.4 test")})
	if rec.Code != 201 {
		t.Fatalf("upload = %d %s", rec.Code, rec.Body)
	}
	a := decodeAs[store.Attachment](t, rec)
	if file := mustCall(t, h, c, "GET", fmt.Sprintf("/api/attachments/%d/file", a.ID), "", 200); file.Body.String() != "%PDF-1.4 test" {
		t.Errorf("file = %q", file.Body)
	}
	mustCall(t, h, c, "PATCH", fmt.Sprintf("/api/attachments/%d", a.ID), `{"kind":"manual","filename":"Boiler manual.pdf"}`, 200)
	mustCall(t, h, c, "POST", fmt.Sprintf("/api/things/%d/attachments/paperless", it.ID), `{"documentIds":[1]}`, 400)

	soon := time.Now().AddDate(0, 0, 3).Format(store.DateLayout)
	later := time.Now().AddDate(0, 0, 90).Format(store.DateLayout)
	m := decodeAs[store.Maintenance](t, mustCall(t, h, c, "POST", fmt.Sprintf("/api/things/%d/maintenance", it.ID),
		fmt.Sprintf(`{"title":"Yearly service","nextDue":%q}`, soon), 201))
	mustCall(t, h, c, "POST", fmt.Sprintf("/api/things/%d/maintenance", it.ID), fmt.Sprintf(`{"title":"Descale","nextDue":%q,"cost":40}`, later), 201)
	mustCall(t, h, c, "POST", fmt.Sprintf("/api/things/%d/maintenance", it.ID), `{"title":"No date"}`, 400)
	due := decodeAs[[]store.Maintenance](t, mustCall(t, h, c, "GET", "/api/maintenance/due", "", 200))
	if len(due) != 1 || due[0].ID != m.ID || due[0].ThingName != "Boiler" {
		t.Errorf("due = %+v", due)
	}
	today := time.Now().Format(store.DateLayout)
	mustCall(t, h, c, "PUT", fmt.Sprintf("/api/maintenance/%d", m.ID), fmt.Sprintf(`{"title":"Yearly service","doneOn":%q}`, today), 200)
	if due := decodeAs[[]store.Maintenance](t, mustCall(t, h, c, "GET", "/api/maintenance/due", "", 200)); len(due) != 0 {
		t.Errorf("due after done = %+v", due)
	}
	got := decodeAs[thingResponse](t, mustCall(t, h, c, "GET", fmt.Sprintf("/api/things/%d", it.ID), "", 200))
	if len(got.Attachments) != 1 || got.Attachments[0].Filename != "Boiler manual.pdf" || len(got.Maintenance) != 2 {
		t.Errorf("thing = %+v", got)
	}
	mustCall(t, h, c, "DELETE", fmt.Sprintf("/api/attachments/%d", a.ID), "", 204)
	mustCall(t, h, c, "GET", fmt.Sprintf("/api/attachments/%d/file", a.ID), "", 404)
}

func TestNotificationSettings(t *testing.T) {
	h, c := newTestServer(t)
	mustCall(t, h, c, "PUT", "/api/settings", `{"theme":"system","notifyUrls":"ntfy://ntfy.sh/wherehouse","notifyDays":"7, 1"}`, 200)
	mustCall(t, h, c, "PUT", "/api/settings", `{"theme":"system","notifyUrls":"nope://x","notifyDays":"7"}`, 400)
	mustCall(t, h, c, "PUT", "/api/settings", `{"theme":"system","notifyDays":"soon"}`, 400)
	if got := decodeAs[settingsResponse](t, mustCall(t, h, c, "GET", "/api/settings", "", 200)); got.NotifyDays != "7, 1" || got.PaperlessEnabled {
		t.Errorf("settings = %+v", got)
	}
}
