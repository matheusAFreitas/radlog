// Package synctest tem um Google Drive de mentira (só o que a nuvem usa) para
// os testes.
package synctest

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"time"
)

// File é um arquivo do Drive de mentira.
type File struct {
	ID, Name, Parent, Mime string
	Data                   []byte
	Created, Modified      time.Time
}

// Server é o Drive de mentira, com o endpoint de token junto.
type Server struct {
	*httptest.Server
	mu      sync.Mutex
	Files   map[string]*File
	Trashed map[string]*File // na lixeira (com o que estava dentro)
	ups     map[string]*File // envios retomáveis abertos
	n       int
	Revoked bool          // o token responde invalid_grant
	Offline bool          // tudo responde 503
	Skew    time.Duration // somado à hora do cabeçalho Date
	Quota   bool          // envio responde 403 de cota cheia
	OnPut   func()        // chamado depois de cada envio terminado (resposta já enviada)
	Calls   []string
}

// New sobe o servidor.
func New() *Server {
	s := &Server{Files: map[string]*File{}, Trashed: map[string]*File{}, ups: map[string]*File{}}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

// Base, Upload e TokenURL são os endereços para o cliente.
func (s *Server) Base() string     { return s.URL + "/drive/v3" }
func (s *Server) Upload() string   { return s.URL + "/upload/drive/v3" }
func (s *Server) TokenURL() string { return s.URL + "/token" }

// Named devolve os arquivos com esse nome.
func (s *Server) Named(name string) []*File {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*File
	for _, f := range s.Files {
		if f.Name == name {
			out = append(out, f)
		}
	}
	return out
}

// Count devolve quantos arquivos (fora pastas) existem.
func (s *Server) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, f := range s.Files {
		if f.Mime == "" {
			n++
		}
	}
	return n
}

var (
	reName   = regexp.MustCompile(`name = '([^']*)'`)
	reParent = regexp.MustCompile(`'([^']*)' in parents`)
)

func (s *Server) js(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func meta(f *File) map[string]any {
	sum := md5.Sum(f.Data)
	m := map[string]any{"id": f.ID, "name": f.Name, "modifiedTime": f.Modified.Format(time.RFC3339Nano), "createdTime": f.Created.Format(time.RFC3339Nano)}
	if f.Mime == "" {
		m["size"] = fmt.Sprint(len(f.Data))
		m["md5Checksum"] = hex.EncodeToString(sum[:])
	}
	return m
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Calls = append(s.Calls, r.Method+" "+r.URL.Path)
	w.Header().Set("Date", time.Now().Add(s.Skew).UTC().Format(http.TimeFormat))
	if s.Offline {
		w.WriteHeader(503)
		return
	}
	body, _ := io.ReadAll(r.Body)
	p := r.URL.Path
	now := time.Now()
	switch {
	case p == "/token":
		_ = r.ParseForm()
		if s.Revoked {
			w.WriteHeader(400)
			s.js(w, map[string]string{"error": "invalid_grant"})
			return
		}
		// id_token só com o e-mail (o cliente não confere a assinatura)
		pl := base64.RawURLEncoding.EncodeToString([]byte(`{"email":"voce@example.com"}`))
		s.js(w, map[string]any{"access_token": "at-" + fmt.Sprint(now.UnixNano()), "refresh_token": "rt-1", "expires_in": 3600, "id_token": "x." + pl + ".y"})
	case r.Method == "GET" && p == "/drive/v3/files":
		q := r.URL.Query().Get("q")
		var out []map[string]any
		for _, f := range s.Files {
			if m := reName.FindStringSubmatch(q); m != nil && (f.Name != m[1] || f.Mime == "") {
				continue
			}
			if m := reParent.FindStringSubmatch(q); m != nil && f.Parent != m[1] {
				continue
			}
			out = append(out, meta(f))
		}
		s.js(w, map[string]any{"files": out})
	case r.Method == "POST" && p == "/drive/v3/files":
		var m struct{ Name, MimeType string }
		_ = json.Unmarshal(body, &m)
		s.n++
		f := &File{ID: fmt.Sprintf("f%d", s.n), Name: m.Name, Mime: m.MimeType, Created: now, Modified: now}
		s.Files[f.ID] = f
		s.js(w, meta(f))
	case r.Method == "GET" && strings.HasPrefix(p, "/drive/v3/files/"):
		f := s.Files[strings.TrimPrefix(p, "/drive/v3/files/")]
		if f == nil {
			w.WriteHeader(404)
			return
		}
		_, _ = w.Write(f.Data)
	case r.Method == "PATCH" && strings.HasPrefix(p, "/drive/v3/files/"):
		id := strings.TrimPrefix(p, "/drive/v3/files/")
		f := s.Files[id]
		if f == nil {
			w.WriteHeader(404)
			return
		}
		var m struct{ Trashed bool }
		_ = json.Unmarshal(body, &m)
		if m.Trashed {
			for k, x := range s.Files {
				if k == id || x.Parent == id {
					s.Trashed[k] = x
					delete(s.Files, k)
				}
			}
		}
		s.js(w, meta(f))
	case r.Method == "DELETE" && strings.HasPrefix(p, "/drive/v3/files/"):
		id := strings.TrimPrefix(p, "/drive/v3/files/")
		if s.Files[id] == nil {
			w.WriteHeader(404)
			return
		}
		delete(s.Files, id)
		for k, f := range s.Files {
			if f.Parent == id {
				delete(s.Files, k)
			}
		}
		w.WriteHeader(204)
	case s.Quota && strings.HasPrefix(p, "/upload/"):
		w.WriteHeader(403)
		s.js(w, map[string]any{"error": map[string]any{"code": 403, "message": "The user's Drive storage quota has been exceeded."}})
	case r.Method == "POST" && p == "/upload/drive/v3/files":
		var m struct {
			Name    string
			Parents []string
		}
		_ = json.Unmarshal(body, &m)
		s.n++
		f := &File{ID: fmt.Sprintf("f%d", s.n), Name: m.Name, Created: now}
		if len(m.Parents) > 0 {
			f.Parent = m.Parents[0]
		}
		s.openUp(w, f)
	case r.Method == "PATCH" && strings.HasPrefix(p, "/upload/drive/v3/files/"):
		f := s.Files[strings.TrimPrefix(p, "/upload/drive/v3/files/")]
		if f == nil {
			w.WriteHeader(404)
			return
		}
		s.openUp(w, f)
	case r.Method == "PUT" && strings.HasPrefix(p, "/up/"):
		f := s.ups[p]
		if f == nil {
			w.WriteHeader(404)
			return
		}
		delete(s.ups, p)
		f.Data, f.Modified = body, now
		s.Files[f.ID] = f
		s.js(w, meta(f))
		if s.OnPut != nil {
			if fl, ok := w.(http.Flusher); ok {
				fl.Flush()
			}
			s.OnPut()
		}
	default:
		w.WriteHeader(400)
	}
}

func (s *Server) openUp(w http.ResponseWriter, f *File) {
	s.n++
	loc := fmt.Sprintf("/up/%d", s.n)
	s.ups[loc] = f
	w.Header().Set("Location", s.URL+loc)
	w.WriteHeader(200)
}
