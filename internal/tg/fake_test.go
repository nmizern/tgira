package tg

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// apiCall is one request the bot made to Telegram.
type apiCall struct {
	Method string
	Params map[string]any
}

func (c apiCall) str(key string) string {
	if v, ok := c.Params[key].(string); ok {
		return v
	}
	return ""
}

// fakeAPI stands in for the Bot API: it records what the bot sends and answers
// with plausible results.
type fakeAPI struct {
	srv *httptest.Server

	mu       sync.Mutex
	recorded []apiCall
	failures map[string]string
	nextID   int
}

func newFakeAPI(t *testing.T) *fakeAPI {
	f := &fakeAPI{failures: map[string]string{}, nextID: 5000}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]

	params := map[string]any{}
	_ = json.NewDecoder(r.Body).Decode(&params)

	f.mu.Lock()
	f.recorded = append(f.recorded, apiCall{Method: method, Params: params})
	failure, failed := f.failures[method]
	f.nextID++
	id := f.nextID
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if failed {
		writeJSON(w, map[string]any{"ok": false, "error_code": 400, "description": failure})
		return
	}

	switch method {
	case "sendMessage", "editMessageText":
		if existing, ok := params["message_id"].(string); ok {
			writeJSON(w, okResult(messageResult(existing, params)))
			return
		}
		writeJSON(w, okResult(messageResult(itoa(id), params)))

	case "copyMessage":
		writeJSON(w, okResult(map[string]any{"message_id": id}))

	default:
		writeJSON(w, okResult(true))
	}
}

func messageResult(id string, params map[string]any) map[string]any {
	chat := params["chat_id"]
	if chat == nil {
		chat = "0"
	}
	return map[string]any{
		"message_id": atoi(id),
		"chat":       map[string]any{"id": atoi(toStr(chat)), "type": "supergroup"},
		"text":       params["text"],
		"date":       0,
	}
}

func okResult(result any) map[string]any {
	return map[string]any{"ok": true, "result": result}
}

func writeJSON(w http.ResponseWriter, body any) {
	_ = json.NewEncoder(w).Encode(body)
}

// fail makes the next calls to a method come back as a Telegram error.
func (f *fakeAPI) fail(method, description string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failures[method] = description
}

func (f *fakeAPI) methods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]string, 0, len(f.recorded))
	for _, c := range f.recorded {
		out = append(out, c.Method)
	}
	return out
}

func (f *fakeAPI) calls(method string) []apiCall {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []apiCall
	for _, c := range f.recorded {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

func (f *fakeAPI) last(t *testing.T, method string) apiCall {
	t.Helper()
	calls := f.calls(method)
	if len(calls) == 0 {
		t.Fatalf("no %s call was made; got %v", method, f.methods())
	}
	return calls[len(calls)-1]
}
