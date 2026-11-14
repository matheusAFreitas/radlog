package app

import (
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"radlog/internal/photometa"
)

// ---------- medições de objeto: anexos, comparabilidade e contexto (2.7) ----------

// MeasureAttachment é um arquivo anexado a uma medição (foto do objeto, da
// geometria, um laudo…). Fica em medicoes-anexos/<id>/<arquivo>.
type MeasureAttachment struct {
	File  string `json:"file"` // nome no disco (único dentro da medição)
	Name  string `json:"name"` // nome original
	Size  int64  `json:"size"`
	Type  string `json:"type"` // tipo MIME
	Added int64  `json:"added"`
}

const (
	attachDirName = "medicoes-anexos"
	// início (s) de cada medição dos anexos de um pacote (rcsync.go)
	attachStartsFile = "medicoes-anexos-inicio.json"
	attachMaxSize    = 25 << 20 // 25 MB por arquivo
	attachMaxView    = 12 << 20 // miniatura/visualização no app
)

func (a *App) attachDir(id int64) string {
	return filepath.Join(a.dataDir, attachDirName, fmt.Sprint(id))
}

var unsafeName = regexp.MustCompile(`[^\pL\pN._ -]+`)

// safeFileName tira separadores e caracteres estranhos do nome.
func safeFileName(n string) string {
	n = filepath.Base(strings.ReplaceAll(n, "\\", "/"))
	n = strings.TrimSpace(unsafeName.ReplaceAllString(n, "_"))
	n = strings.TrimLeftFunc(n, func(r rune) bool { return r == '.' || unicode.IsSpace(r) })
	if n == "" {
		n = "anexo"
	}
	if len(n) > attachNameMax {
		ext := filepath.Ext(n)
		if len(ext) > 10 {
			ext = ""
		}
		n = cutUTF8(n, attachNameMax-len(ext)) + ext
	}
	return n
}

// attachNameMax é o tamanho máximo (em bytes) do nome de um anexo.
const attachNameMax = 80

// cutUTF8 corta s em até n bytes sem partir uma letra acentuada no meio.
func cutUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// numberedName é name com "-k" antes da extensão, cortando o nome para
// caber em attachNameMax: o resultado passa no safeFileName sem mudar
// (QA 13/11: nome de 80 caracteres anexado duas vezes sumia da lista).
func numberedName(name string, k int) string {
	ext := filepath.Ext(name)
	suf := fmt.Sprintf("-%d%s", k, ext)
	return cutUTF8(strings.TrimSuffix(name, ext), attachNameMax-len(suf)) + suf
}

// attachFiles copia os arquivos para a medição id e devolve os anexos dela.
func (a *App) attachFiles(id int64, paths []string) ([]MeasureAttachment, error) {
	a.mu.Lock()
	idx := -1
	for i := range a.measurements {
		if a.measurements[i].ID == id {
			idx = i
		}
	}
	if idx < 0 {
		a.mu.Unlock()
		return nil, errors.New("medição não encontrada")
	}
	used := map[string]bool{}
	for _, at := range a.measurements[idx].Attachments {
		used[strings.ToLower(at.File)] = true
	}
	a.mu.Unlock()

	dir := a.attachDir(id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var added []MeasureAttachment
	var errs []string
	for _, src := range paths {
		st, err := os.Stat(src)
		if err != nil || st.IsDir() {
			errs = append(errs, filepath.Base(src)+": não encontrado")
			continue
		}
		if st.Size() > attachMaxSize {
			errs = append(errs, filepath.Base(src)+": maior que 25 MB")
			continue
		}
		name := safeFileName(filepath.Base(src))
		file := name
		ext := filepath.Ext(name)
		for k := 2; used[strings.ToLower(file)]; k++ {
			file = numberedName(name, k)
		}
		size, err := copyAttachment(src, filepath.Join(dir, file), ext)
		if err != nil {
			errs = append(errs, filepath.Base(src)+": "+err.Error())
			continue
		}
		used[strings.ToLower(file)] = true
		if photometa.Risky(ext) {
			errs = append(errs, filepath.Base(src)+": "+errAttachLocal.Error())
		}
		typ := mime.TypeByExtension(strings.ToLower(ext))
		if typ == "" {
			typ = "application/octet-stream"
		}
		added = append(added, MeasureAttachment{File: file, Name: filepath.Base(src), Size: size, Type: typ, Added: time.Now().UnixMilli()})
	}
	a.mu.Lock()
	var out []MeasureAttachment
	for i := range a.measurements {
		if a.measurements[i].ID == id {
			a.measurements[i].Attachments = append(a.measurements[i].Attachments, added...)
			out = append(out, a.measurements[i].Attachments...)
		}
	}
	err := a.saveMeasurementsLocked()
	a.mu.Unlock()
	a.emit("measurements", a.GetMeasurements())
	if err == nil && len(errs) > 0 {
		err = errors.New(strings.Join(errs, "; "))
	}
	return out, err
}

// AddMeasurementAttachment abre a janela para escolher fotos ou arquivos e
// anexa à medição.
func (a *App) AddMeasurementAttachment(id int64) ([]MeasureAttachment, error) {
	paths, err := wruntime.OpenMultipleFilesDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Anexar à medição",
		Filters: []wruntime.FileFilter{
			{DisplayName: "Fotos e documentos", Pattern: "*.jpg;*.jpeg;*.png;*.webp;*.gif;*.heic;*.pdf;*.txt;*.csv"},
			{DisplayName: "Todos os arquivos", Pattern: "*.*"},
		},
	})
	if err != nil || len(paths) == 0 {
		return a.measurementAttachments(id), err
	}
	return a.attachFiles(id, paths)
}

func (a *App) measurementAttachments(id int64) []MeasureAttachment {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, m := range a.measurements {
		if m.ID == id {
			return append([]MeasureAttachment{}, m.Attachments...)
		}
	}
	return nil
}

// cleanAttachments tira da lista os anexos com nome que sairia da pasta
// (ex.: "../../config.json" vindo de um backup de outra pessoa, S1).
func cleanAttachments(list []MeasureAttachment) []MeasureAttachment {
	out := list[:0:0]
	for _, at := range list {
		if at.File != "" && safeFileName(at.File) == at.File {
			out = append(out, at)
		}
	}
	return out
}

// attachOpenExt são os tipos que "Abrir" entrega ao programa do sistema;
// o resto (um .exe vindo de backup, por exemplo) só abre a pasta.
var attachOpenExt = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".heic": true, ".bmp": true,
	".pdf": true, ".txt": true, ".csv": true, ".md": true,
}

// attachPath acha o arquivo do anexo (só os que estão na lista da medição).
func (a *App) attachPath(id int64, file string) (string, MeasureAttachment, error) {
	if file == "" || safeFileName(file) != file {
		return "", MeasureAttachment{}, errors.New("anexo não encontrado")
	}
	for _, at := range a.measurementAttachments(id) {
		if at.File == file {
			return filepath.Join(a.attachDir(id), at.File), at, nil
		}
	}
	return "", MeasureAttachment{}, errors.New("anexo não encontrado")
}

// RemoveMeasurementAttachment apaga um anexo.
func (a *App) RemoveMeasurementAttachment(id int64, file string) error {
	p, _, err := a.attachPath(id, file)
	if err != nil {
		return err
	}
	a.mu.Lock()
	for i := range a.measurements {
		if a.measurements[i].ID != id {
			continue
		}
		kept := a.measurements[i].Attachments[:0]
		for _, at := range a.measurements[i].Attachments {
			if at.File != file {
				kept = append(kept, at)
			}
		}
		a.measurements[i].Attachments = kept
	}
	err = a.saveMeasurementsLocked()
	a.mu.Unlock()
	_ = os.Remove(p)
	a.emit("measurements", a.GetMeasurements())
	return err
}

// GetMeasurementAttachment devolve o anexo como data URL (para mostrar fotos).
func (a *App) GetMeasurementAttachment(id int64, file string) (string, error) {
	p, at, err := a.attachPath(id, file)
	if err != nil {
		return "", err
	}
	if at.Size > attachMaxView {
		return "", errors.New("arquivo grande demais para mostrar aqui; use Abrir")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return "data:" + at.Type + ";base64," + base64.StdEncoding.EncodeToString(b), nil
}

// OpenMeasurementAttachment abre o anexo no programa padrão do sistema.
func (a *App) OpenMeasurementAttachment(id int64, file string) error {
	p, _, err := a.attachPath(id, file)
	if err != nil {
		return err
	}
	if !attachOpenExt[strings.ToLower(filepath.Ext(p))] {
		return openFolder(filepath.Dir(p))
	}
	return openFolder(p)
}

// SetMeasurementComparable marca (ou desmarca) uma medição como não
// comparável com as outras do mesmo objeto, com o motivo.
func (a *App) SetMeasurementComparable(id int64, comparable bool, reason string) error {
	a.mu.Lock()
	found := false
	for i := range a.measurements {
		if a.measurements[i].ID == id {
			a.measurements[i].NotComparable = !comparable
			a.measurements[i].NCReason = ""
			if !comparable {
				a.measurements[i].NCReason = strings.TrimSpace(reason)
			}
			found = true
		}
	}
	var err error
	if !found {
		err = errors.New("medição não encontrada")
	} else {
		err = a.saveMeasurementsLocked()
	}
	a.mu.Unlock()
	a.emit("measurements", a.GetMeasurements())
	return err
}

// ---------- contexto: configuração da época e as outras do mesmo objeto ----------

// MeasureCfgItem é um ajuste do detector na época da medição.
type MeasureCfgItem struct {
	Field string `json:"field"`
	Then  string `json:"then"` // "" = sem registro
	Now   string `json:"now"`
}

// MeasureRelated é outra medição do mesmo objeto (mesmo nome).
type MeasureRelated struct {
	ID            int64   `json:"id"`
	Time          int64   `json:"time"`
	Device        string  `json:"device"`
	NetUSv        float64 `json:"netUSv"`
	NetErr        float64 `json:"netErr"` // CPM (GMC) ou cps (RC102), como no resultado
	NetErrUSv     float64 `json:"netErrUSv"`
	Distance      string  `json:"distance"`
	NotComparable bool    `json:"notComparable"`
	NCReason      string  `json:"ncReason"`
	// por que não dá para comparar direto com a escolhida (vazio = dá)
	Diff []string `json:"diff"`
}

// MeasureContext é o que a tela mostra junto da medição.
type MeasureContext struct {
	Cfg      []MeasureCfgItem `json:"cfg"`
	CfgKnown bool             `json:"cfgKnown"` // o histórico de configurações cobre a data
	Changed  []string         `json:"changed"`  // ajustes que mudaram desde a medição
	Related  []MeasureRelated `json:"related"`  // mais nova primeiro
}

// campos do histórico que mudam o resultado de uma medição do GMC
var measureCfgFields = []string{"Calibração em uso", "Tempo morto", "Calibração do GMC"}

func measureNameKey(n string) string { return strings.Join(strings.Fields(strings.ToLower(n)), " ") }

// GetMeasurementContext devolve a configuração do detector na época, o que
// mudou desde então e as outras medições do mesmo objeto, dizendo quais não
// dá para comparar direto (marcadas, outro aparelho, outra distância, outra
// configuração).
func (a *App) GetMeasurementContext(id int64) (MeasureContext, error) {
	a.mu.Lock()
	var m *SavedMeasure
	all := append([]SavedMeasure{}, a.measurements...)
	for i := range all {
		if all[i].ID == id {
			m = &all[i]
		}
	}
	now := a.cfgSnapshotLocked()
	a.mu.Unlock()
	if m == nil {
		return MeasureContext{}, errors.New("medição não encontrada")
	}
	a.cfgMu.Lock()
	hist := a.readCfgHist()
	a.cfgMu.Unlock()
	cfgAt := func(t int64) map[string]string {
		out := map[string]string{}
		for _, f := range measureCfgFields {
			if v, ok := cfgValueAt(hist, f, t); ok {
				out[f] = v
			}
		}
		return out
	}
	ctx := MeasureContext{Cfg: []MeasureCfgItem{}, Changed: []string{}, Related: []MeasureRelated{}}
	rc := m.Device == devRC
	if !rc {
		then := cfgAt(m.Obj.End)
		ctx.CfgKnown = len(then) > 0
		for _, f := range measureCfgFields {
			it := MeasureCfgItem{Field: f, Then: then[f], Now: now[f]}
			ctx.Cfg = append(ctx.Cfg, it)
			if it.Then != "" && it.Then != it.Now {
				ctx.Changed = append(ctx.Changed, f)
			}
		}
	}
	mine := cfgAt(m.Obj.End)
	key := measureNameKey(m.Name)
	for _, o := range all {
		if o.ID == m.ID || measureNameKey(o.Name) != key {
			continue
		}
		r := MeasureRelated{ID: o.ID, Time: o.Time, Device: o.Device, NetUSv: o.Result.NetUSv, NetErr: o.Result.NetErr,
			Distance: o.Distance, NotComparable: o.NotComparable, NCReason: o.NCReason, Diff: []string{}}
		if o.Result.NetCPM != 0 {
			r.NetErrUSv = o.Result.NetErr * o.Result.NetUSv / o.Result.NetCPM
		}
		if o.NotComparable {
			r.Diff = append(r.Diff, "marcada como não comparável")
		}
		if m.NotComparable {
			r.Diff = append(r.Diff, "esta medição está marcada como não comparável")
		}
		if o.Device != m.Device {
			r.Diff = append(r.Diff, "outro aparelho")
		}
		if strings.TrimSpace(strings.ToLower(o.Distance)) != strings.TrimSpace(strings.ToLower(m.Distance)) {
			r.Diff = append(r.Diff, "outra distância")
		}
		if o.Device != devRC && m.Device != devRC {
			theirs := cfgAt(o.Obj.End)
			for _, f := range measureCfgFields {
				if mine[f] != "" && theirs[f] != "" && mine[f] != theirs[f] {
					r.Diff = append(r.Diff, "outra configuração ("+strings.ToLower(f)+")")
				}
			}
		}
		ctx.Related = append(ctx.Related, r)
	}
	sort.Slice(ctx.Related, func(i, j int) bool { return ctx.Related[i].Time > ctx.Related[j].Time })
	return ctx, nil
}

// errPhotoMeta: a foto não foi anexada porque não deu para tirar a
// localização de dentro dela.
var errPhotoMeta = errors.New("não consegui tirar a localização e os dados da câmera de dentro desta foto, então ela não foi anexada")

// errAttachLocal não é erro: avisa que o anexo foi guardado, mas fica só
// neste PC (S1).
var errAttachLocal = errors.New("anexado, mas fica só neste PC: esse tipo de arquivo pode ter a localização dentro e o app não sabe tirar, então ele não vai para o backup nem para a nuvem")

// copyAttachment copia o anexo para a pasta do app. Fotos vão sem os
// metadados (localização do celular, modelo, data): eles não podem ir para
// o backup nem para a nuvem (auditoria 2.76.0, S2). Devolve o tamanho
// gravado.
func copyAttachment(src, dst, ext string) (int64, error) {
	if !photometa.Handles(ext) {
		return copyFile(src, dst)
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return 0, err
	}
	clean, err := photometa.Strip(b, ext)
	if err != nil {
		return 0, errPhotoMeta
	}
	return int64(len(clean)), writeFileAtomic(dst, clean, 0o644)
}

// attachForExport tira os metadados de uma foto anexada antes de ela sair
// do PC (backup e sincronização): as anexadas antes da 2.77.0 ainda têm a
// localização. Devolve false se a foto deve ficar de fora.
func (a *App) attachForExport(rel string, b []byte) ([]byte, bool) {
	ext := filepath.Ext(rel)
	if photometa.Risky(ext) {
		return nil, false // fica só no PC (S1)
	}
	if !photometa.Handles(ext) {
		return b, true
	}
	clean, err := photometa.Strip(b, ext)
	if err != nil {
		a.logf("anexo %s ficou de fora (não consegui tirar a localização de dentro da foto)", rel)
		return nil, false
	}
	return clean, true
}
