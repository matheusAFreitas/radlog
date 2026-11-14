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
	"sort"
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
	Created  time.Time `json:"createdTime"`
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
	case *limited:
		if resp.ContentLength > o.max {
			return resp.Header, ErrTooBig
		}
		*o.b, err = io.ReadAll(io.LimitReader(resp.Body, o.max+1))
		if err == nil && int64(len(*o.b)) > o.max {
			*o.b = nil
			return resp.Header, ErrTooBig
		}
	default:
		err = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.Header, err
}

func qs(s string) string {
	return "'" + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), "'", `\'`) + "'"
}

// Marca da pasta do app (appProperties): acha a pasta mesmo que a pessoa
// mude o nome dela no Drive (auditoria 2.79.0, N17).
const (
	FolderPropKey = "radlog"
	FolderPropVal = "nuvem"
)

// FindFolders acha as pastas do app, pela marca ou pelo nome (as de antes da
// marca), da mais antiga para a mais nova.
func (d *Drive) FindFolders(ctx context.Context, name string) ([]File, error) {
	seen := map[string]bool{}
	var out []File
	for _, cond := range []string{
		fmt.Sprintf("appProperties has { key=%s and value=%s }", qs(FolderPropKey), qs(FolderPropVal)),
		fmt.Sprintf("name = %s", qs(name)),
	} {
		q := url.Values{
			"q":        {fmt.Sprintf("%s and mimeType = %s and trashed = false", cond, qs(folderMime))},
			"fields":   {"files(id,name,createdTime)"},
			"orderBy":  {"createdTime"},
			"pageSize": {"20"},
			"spaces":   {"drive"},
		}
		var r struct {
			Files []File `json:"files"`
		}
		if _, err := d.do(ctx, "GET", d.Base+"/files?"+q.Encode(), "", nil, &r); err != nil {
			return nil, err
		}
		for _, f := range r.Files {
			if !seen[f.ID] {
				seen[f.ID] = true
				out = append(out, f)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Created.Before(out[j].Created) })
	return out, nil
}

// FindFolder acha a pasta do app mais antiga ("" se não existe).
func (d *Drive) FindFolder(ctx context.Context, name string) (string, error) {
	fs, err := d.FindFolders(ctx, name)
	if err != nil || len(fs) == 0 {
		return "", err
	}
	return fs[0].ID, nil
}

// CreateFolder cria a pasta na raiz do Drive, já com a marca.
func (d *Drive) CreateFolder(ctx context.Context, name string) (string, error) {
	b, _ := json.Marshal(map[string]any{"name": name, "mimeType": folderMime, "appProperties": map[string]string{FolderPropKey: FolderPropVal}})
	var f File
	if _, err := d.do(ctx, "POST", d.Base+"/files?fields=id", "application/json", b, &f); err != nil {
		return "", err
	}
	return f.ID, nil
}

// TagFolder põe a marca numa pasta criada antes dela existir.
func (d *Drive) TagFolder(ctx context.Context, id string) error {
	b, _ := json.Marshal(map[string]any{"appProperties": map[string]string{FolderPropKey: FolderPropVal}})
	_, err := d.do(ctx, "PATCH", d.Base+"/files/"+url.PathEscape(id)+"?fields=id", "application/json", b, nil)
	return err
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

// MaxDownload é o maior arquivo que Get aceita: o maior pacote do app (um
// anexo de 25 MB ou um mês de espectros) cabe com folga (auditoria 2.79.0,
// N14).
var MaxDownload int64 = 256 << 20

// ErrTooBig é um arquivo maior que MaxDownload.
var ErrTooBig = errors.New("arquivo grande demais para baixar")

// Get baixa o conteúdo de um arquivo, até MaxDownload.
func (d *Drive) Get(ctx context.Context, id string) ([]byte, error) {
	var b []byte
	_, err := d.do(ctx, "GET", d.Base+"/files/"+url.PathEscape(id)+"?alt=media", "", nil, &limited{max: MaxDownload, b: &b})
	return b, err
}

// limited é o destino de do() que para de ler depois de max bytes.
type limited struct {
	max int64
	b   *[]byte
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
	if len(data) <= ChunkSize {
		var f File
		_, err = d.do(ctx, "PUT", loc, "application/octet-stream", data, &f)
		return f, err
	}
	return d.putChunks(ctx, loc, data)
}

// ChunkSize é o tamanho de cada pedaço do envio (múltiplo de 256 KiB, como o
// Drive pede). Arquivo maior sobe em pedaços: se a conexão cair, o envio
// continua do último pedaço que chegou, em vez de recomeçar (N14).
var ChunkSize = 8 << 20

// chunkTries é quantas vezes seguidas um pedaço pode falhar.
const chunkTries = 5

// putChunks sobe data em pedaços no envio aberto em loc.
func (d *Drive) putChunks(ctx context.Context, loc string, data []byte) (File, error) {
	total := len(data)
	off, fails := 0, 0
	for {
		end := off + ChunkSize
		if end > total {
			end = total
		}
		f, next, err := d.putRange(ctx, loc, data[off:end], fmt.Sprintf("bytes %d-%d/%d", off, end-1, total))
		if err == nil && next < 0 {
			return f, nil
		}
		if err != nil {
			var he *HTTPError
			if ctx.Err() != nil || (errors.As(err, &he) && he.Code/100 == 4) || errors.Is(err, ErrNotFound) {
				return File{}, err
			}
			if fails++; fails >= chunkTries {
				return File{}, err
			}
			// pergunta ao Drive até onde chegou e continua dali
			select {
			case <-ctx.Done():
				return File{}, ctx.Err()
			case <-time.After(time.Duration(fails) * time.Second):
			}
			f, next, err = d.putRange(ctx, loc, nil, fmt.Sprintf("bytes */%d", total))
			if err != nil {
				continue
			}
			if next < 0 {
				return f, nil
			}
		} else {
			fails = 0
		}
		off = next
	}
}

// putRange manda um pedaço. next é o primeiro byte que falta (-1 = terminou,
// e f é o arquivo).
func (d *Drive) putRange(ctx context.Context, loc string, body []byte, crange string) (f File, next int, err error) {
	tok, err := d.Token(ctx)
	if err != nil {
		return File{}, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, "PUT", loc, bytes.NewReader(body))
	if err != nil {
		return File{}, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Range", crange)
	req.ContentLength = int64(len(body))
	resp, err := d.HTTP.Do(req)
	if err != nil {
		return File{}, 0, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == 308:
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		// "bytes=0-N": chegou até N; sem Range, nada chegou
		next = 0
		if r := resp.Header.Get("Range"); r != "" {
			var a, b int
			if _, err := fmt.Sscanf(r, "bytes=%d-%d", &a, &b); err == nil {
				next = b + 1
			}
		}
		return File{}, next, nil
	case resp.StatusCode == 404:
		return File{}, 0, ErrNotFound
	case resp.StatusCode/100 == 2:
		err = json.NewDecoder(resp.Body).Decode(&f)
		return f, -1, err
	default:
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(b, &e)
		return File{}, 0, &HTTPError{Code: resp.StatusCode, Msg: e.Error.Message}
	}
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
