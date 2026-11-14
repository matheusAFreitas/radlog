package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ---------- Google Drive (REST v3) ----------
//
// Só o que a nuvem usa: achar/criar a pasta, listar, baixar, subir (criar ou
// trocar o conteúdo) e apagar. Com o escopo drive.file o app só vê o que ele
// mesmo criou.

const folderMime = "application/vnd.google-apps.folder"

// Drive é o cliente. Token devolve um token de acesso válido; OnDate recebe a
// hora do servidor de cada resposta (para avisar de relógio errado).
type Drive struct {
	HTTP   *http.Client
	Token  func(ctx context.Context) (string, error)
	Base   string // https://www.googleapis.com/drive/v3
	Upload string // https://www.googleapis.com/upload/drive/v3
	OnDate func(t time.Time)
}

// NewDrive devolve o cliente com os endereços do Google.
func NewDrive(token func(ctx context.Context) (string, error)) *Drive {
	return &Drive{
		HTTP:   &http.Client{Timeout: 10 * time.Minute},
		Token:  token,
		Base:   "https://www.googleapis.com/drive/v3",
		Upload: "https://www.googleapis.com/upload/drive/v3",
	}
}

// File é um arquivo da pasta.
type File struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Size     int64     `json:"size,string,omitempty"`
	MD5      string    `json:"md5Checksum,omitempty"`
	Modified time.Time `json:"modifiedTime"`
}

const fileFields = "id,name,size,md5Checksum,modifiedTime"

// ErrNotFound é o 404 do Drive.
var ErrNotFound = errors.New("arquivo não está mais no Drive")

func (d *Drive) do(ctx context.Context, method, u, ctype string, body []byte, out any) (http.Header, error) {
	tok, err := d.Token(ctx)
	if err != nil {
		return nil, err
	}
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	resp, err := d.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if d.OnDate != nil {
		if t, err := http.ParseTime(resp.Header.Get("Date")); err == nil {
			d.OnDate(t)
		}
	}
	if resp.StatusCode == 404 {
		return nil, ErrNotFound
	}
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(b, &e)
		return nil, &HTTPError{Code: resp.StatusCode, Msg: e.Error.Message}
	}
	switch o := out.(type) {
	case nil:
		_, _ = io.Copy(io.Discard, resp.Body)
	case *[]byte:
		*o, err = io.ReadAll(resp.Body)
	default:
		err = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.Header, err
}

func qs(s string) string {
	return "'" + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), "'", `\'`) + "'"
}

// FindFolder acha a pasta do app na raiz pelo nome ("" se não existe).
func (d *Drive) FindFolder(ctx context.Context, name string) (string, error) {
	q := url.Values{
		"q":        {fmt.Sprintf("name = %s and mimeType = %s and trashed = false", qs(name), qs(folderMime))},
		"fields":   {"files(id,name,createdTime)"},
		"orderBy":  {"createdTime"},
		"pageSize": {"10"},
		"spaces":   {"drive"},
	}
	var r struct {
		Files []File `json:"files"`
	}
	if _, err := d.do(ctx, "GET", d.Base+"/files?"+q.Encode(), "", nil, &r); err != nil {
		return "", err
	}
	if len(r.Files) == 0 {
		return "", nil
	}
	return r.Files[0].ID, nil
}

// CreateFolder cria a pasta na raiz do Drive.
func (d *Drive) CreateFolder(ctx context.Context, name string) (string, error) {
	b, _ := json.Marshal(map[string]any{"name": name, "mimeType": folderMime})
	var f File
	if _, err := d.do(ctx, "POST", d.Base+"/files?fields=id", "application/json", b, &f); err != nil {
		return "", err
	}
	return f.ID, nil
}

// List lista os arquivos da pasta.
func (d *Drive) List(ctx context.Context, folder string) ([]File, error) {
	var out []File
	page := ""
	for {
		q := url.Values{
			"q":        {fmt.Sprintf("%s in parents and trashed = false", qs(folder))},
			"fields":   {"nextPageToken,files(" + fileFields + ")"},
			"pageSize": {"1000"},
			"spaces":   {"drive"},
		}
		if page != "" {
			q.Set("pageToken", page)
		}
		var r struct {
			Next  string `json:"nextPageToken"`
			Files []File `json:"files"`
		}
		if _, err := d.do(ctx, "GET", d.Base+"/files?"+q.Encode(), "", nil, &r); err != nil {
			return nil, err
		}
		out = append(out, r.Files...)
		if r.Next == "" {
			return out, nil
		}
		page = r.Next
	}
}

// Get baixa o conteúdo de um arquivo.
func (d *Drive) Get(ctx context.Context, id string) ([]byte, error) {
	var b []byte
	_, err := d.do(ctx, "GET", d.Base+"/files/"+url.PathEscape(id)+"?alt=media", "", nil, &b)
	return b, err
}

// Put sobe data: cria o arquivo na pasta (id vazio) ou troca o conteúdo do
// arquivo id. Usa o envio retomável, que aceita arquivos grandes.
func (d *Drive) Put(ctx context.Context, folder, id, name string, data []byte) (File, error) {
	meta := map[string]any{}
	method, u := "PATCH", d.Upload+"/files/"+url.PathEscape(id)
	if id == "" {
		meta["name"], meta["parents"] = name, []string{folder}
		method, u = "POST", d.Upload+"/files"
	}
	mb, _ := json.Marshal(meta)
	h, err := d.do(ctx, method, u+"?uploadType=resumable&fields="+fileFields, "application/json; charset=UTF-8", mb, nil)
	if err != nil {
		return File{}, err
	}
	loc := h.Get("Location")
	if loc == "" {
		return File{}, errors.New("o Drive não devolveu o endereço do envio")
	}
	var f File
	_, err = d.do(ctx, "PUT", loc, "application/octet-stream", data, &f)
	return f, err
}

// Trash põe o arquivo ou a pasta (com o que tem dentro) na lixeira do Drive:
// a pessoa ainda recupera por 30 dias (auditoria 2.79.0, N11).
func (d *Drive) Trash(ctx context.Context, id string) error {
	_, err := d.do(ctx, "PATCH", d.Base+"/files/"+url.PathEscape(id)+"?fields=id", "application/json", []byte(`{"trashed":true}`), nil)
	if err == ErrNotFound {
		return nil
	}
	return err
}

// Delete apaga um arquivo (ou a pasta).
func (d *Drive) Delete(ctx context.Context, id string) error {
	_, err := d.do(ctx, "DELETE", d.Base+"/files/"+url.PathEscape(id), "", nil, nil)
	if err == ErrNotFound {
		return nil
	}
	return err
}
