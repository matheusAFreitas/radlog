package app

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"radlog/internal/photometa"
	rs "radlog/internal/sync"
)

// ---------- envio e recebimento no formato 2 (auditoria 2.79.0, passo 2) ----------
//
// Cada aparelho escreve só os arquivos com o prefixo dele (docs/nuvem-formato.md):
//   - "aparelho": nome, último envio e quem escreve (para achar PC copiado);
//   - "comum": os pequenos (alertas encerrados, saídas, medições, anotações,
//     apagados, históricos), igual se nada mudou;
//   - "dia|AAAA-MM-DD": minutos do GMC-800 e do RC102 e segundos de alerta
//     daquele dia (UTC). Só o dia que mudou sobe de novo (N7);
//   - "espectros|AAAA-MM": os espectros fechados do mês;
//   - "anexo|<UID da medição>/<arquivo>": um anexo, que sobe uma vez só.

const (
	cloudKeyDevice = "aparelho"
	cloudKeyCommon = "comum"
	// marca de um pacote que já está na nuvem pelo aparelho que mandou: não
	// sobe de novo daqui (eco)
	cloudEcho = "e:"
)

type cloudPack struct {
	key    string
	kind   byte
	month  string // para a fila na tela
	digest string
	req    pullReq
	att    *cloudAtt
}

type cloudAtt struct {
	id, start int64
	uid, file string
}

// cloudPacks monta a lista de pacotes deste PC e o resumo de cada um.
func (a *App) cloudPacks() ([]cloudPack, error) {
	days, err := a.dayDigests()
	if err != nil {
		return nil, err
	}
	rc := a.rcInventory()
	byDay := map[string]*cloudPack{}
	parts := map[string][]string{}
	day := func(d string) *cloudPack {
		p := byDay[d]
		if p == nil {
			p = &cloudPack{key: "dia|" + d, kind: rs.KindDay, month: d[:7], req: pullReq{Rc: true, cloud: true, noSmall: true}}
			byDay[d] = p
		}
		return p
	}
	for d, h := range days {
		if len(d) != 10 {
			continue
		}
		p := day(d)
		p.req.Days = append(p.req.Days, d)
		parts[d] = append(parts[d], "g"+h)
	}
	for k, h := range rc.Days {
		_, d, ok := strings.Cut(k, "|")
		if !ok || len(d) != 10 {
			continue
		}
		p := day(d)
		p.req.RcDays = append(p.req.RcDays, k)
		parts[d] = append(parts[d], "r"+k+h)
	}
	for _, n := range a.alertFiles() {
		s, _, _ := strings.Cut(strings.TrimSuffix(n, ".json"), "-")
		sec, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			continue
		}
		d := time.Unix(sec, 0).UTC().Format("2006-01-02")
		p := day(d)
		p.req.Alerts = append(p.req.Alerts, n)
		parts[d] = append(parts[d], "a"+n)
	}
	// o comum: o resumo é o próprio pacote (pequeno e igual se nada mudou)
	common := cloudPack{key: cloudKeyCommon, kind: rs.KindCommon, month: time.Now().UTC().Format("2006-01"), req: pullReq{Rc: true, cloud: true}}
	z, err := a.buildDelta(common.req)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(z)
	common.digest = hex.EncodeToString(sum[:16])
	out := []cloudPack{common}
	var ds []string
	for d := range byDay {
		ds = append(ds, d)
	}
	sort.Strings(ds)
	for _, d := range ds {
		p := byDay[d]
		ps := parts[d]
		sort.Strings(ps)
		h := sha256.Sum256([]byte(strings.Join(ps, "\n")))
		p.digest = hex.EncodeToString(h[:16])
		out = append(out, *p)
	}
	// espectros por mês (os sem data no nome ficam juntos)
	spec := map[string][]string{}
	for _, f := range rc.Files {
		m := reMonth.FindString(f)
		if m == "" {
			m = "sem-data"
		}
		spec[m] = append(spec[m], f)
	}
	var ms []string
	for m := range spec {
		ms = append(ms, m)
	}
	sort.Strings(ms)
	for _, m := range ms {
		fs := spec[m]
		sort.Strings(fs)
		h := sha256.Sum256([]byte(strings.Join(fs, "\n")))
		month := m
		if m == "sem-data" {
			month = ""
		}
		out = append(out, cloudPack{key: "espectros|" + m, kind: rs.KindSpectra, month: month, digest: hex.EncodeToString(h[:16]),
			req: pullReq{Rc: true, cloud: true, noSmall: true, RcFiles: fs}})
	}
	// anexos: um arquivo cada, sobe uma vez
	a.mu.Lock()
	for _, m := range a.measurements {
		for _, at := range m.Attachments {
			if m.UID == "" || photometa.Risky(filepath.Ext(at.File)) {
				continue // tipo que pode levar a localização fica no PC (S1)
			}
			out = append(out, cloudPack{key: "anexo|" + m.UID + "/" + at.File, kind: rs.KindAttachment,
				month: time.UnixMilli(m.Time).UTC().Format("2006-01"), digest: "1",
				att: &cloudAtt{id: m.ID, start: m.Time / 1000, uid: m.UID, file: at.File}})
		}
	}
	a.mu.Unlock()
	return out, nil
}

var reMonth = regexp.MustCompile(`\d{4}-\d{2}`)

func cloudQueue(todo []cloudPack) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range todo {
		if p.month != "" && !seen[p.month] {
			seen[p.month] = true
			out = append(out, p.month)
		}
	}
	sort.Strings(out)
	return out
}

// cloudPackData monta o conteúdo (antes de cifrar) de um pacote.
func (a *App) cloudPackData(p cloudPack) ([]byte, error) {
	if p.att != nil {
		rel := fmt.Sprintf("%s/%d/%s", attachDirName, p.att.id, p.att.file)
		b, err := os.ReadFile(filepath.Join(a.dataDir, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		b, ok := a.attachForExport(rel, b) // sem metadados da foto
		if !ok {
			return nil, errors.New("não consegui tirar os metadados da foto")
		}
		return rs.Wrap(p.kind, rs.WrapAttachment(rs.AttachmentHeader{UID: p.att.uid, Start: p.att.start, File: p.att.file}, b)), nil
	}
	z, err := a.buildDelta(p.req)
	if err != nil {
		return nil, err
	}
	// os outros aparelhos não baixam acima do limite: melhor avisar aqui do
	// que o pacote sumir em silêncio lá (QA da 2.80.5)
	if int64(len(z)) > rs.MaxDownload {
		return nil, fmt.Errorf("o pacote %s tem %d MB, acima do limite de %d MB da nuvem, e não subiu", p.key, len(z)>>20, rs.MaxDownload>>20)
	}
	return rs.Wrap(p.kind, z), nil
}

// cloudHold trava e confere que a nuvem ainda é a da sincronização; false =
// destravada, parar (N8).
func (a *App) cloudHold(gen int) bool {
	a.cloud.mu.Lock()
	if a.cloud.gen != gen || !a.cloud.f.On {
		a.cloud.mu.Unlock()
		return false
	}
	return true
}

// errCloudOld: a nuvem é do formato antigo (2.79.x) e precisa ser começada de
// novo; errCloudDup: este PC parece cópia de outro (a tela pergunta).
var (
	errCloudOld = errors.New("nuvem do formato antigo")
	errCloudDup = errors.New("este PC parece cópia de outro aparelho da nuvem")
	// outro aparelho tirou este da nuvem (CloudRemoveDevice)
	errCloudRemoved = errors.New("este PC foi removido da nuvem")
)

func (a *App) cloudSyncRun(ctx context.Context, gen int) error {
	c := &a.cloud
	if !a.cloudHold(gen) {
		return context.Canceled
	}
	folder, keys, dev, code := c.f.Folder, c.keys, c.f.DevID, c.sec.Code
	prefix := keys.Prefix(dev)
	c.mu.Unlock()
	d := a.cloudDriveWith(a.cloudToken)
	files, err := d.List(ctx, folder)
	if err != nil {
		return err
	}
	byName := map[string]rs.File{}
	for _, f := range files {
		byName[f.Name] = f
	}
	// a nuvem ainda é a mesma (ninguém começou outra)
	mf, ok := byName[rs.MetaFile]
	if !ok {
		return errCloudGone
	}
	if !a.cloudHold(gen) {
		return context.Canceled
	}
	// MetaV: o nuvem.json da 2.79.x já tinha visto a meta antiga; sem ela
	// conferida nesta versão, lê de novo para achar o formato antigo
	metaSeen := c.f.Seen[rs.MetaFile] == mf.MD5 && c.f.MetaV == rs.MetaVersion
	tagged := c.f.Tagged
	c.mu.Unlock()
	// a pasta de antes da marca ganha a marca uma vez: assim um aparelho novo
	// acha a nuvem mesmo que a pasta mude de nome (N17)
	if !tagged {
		if err := d.TagFolder(ctx, folder); err == nil && a.cloudHold(gen) {
			c.f.Tagged = true
			a.cloudSaveLocked()
			c.mu.Unlock()
		}
	}
	if !metaSeen {
		b, err := d.Get(ctx, mf.ID)
		if err != nil {
			return err
		}
		var m rs.Meta
		if json.Unmarshal(b, &m) != nil {
			return errCloudGone
		}
		if _, err := m.Unlock(code); errors.Is(err, rs.ErrOldCloud) {
			return errCloudOld
		} else if err != nil {
			return errCloudGone
		}
		if a.cloudHold(gen) {
			c.f.MetaV = rs.MetaVersion
			c.mu.Unlock()
		}
		a.cloudSeen(gen, rs.MetaFile, mf.MD5)
	}

	// 0) o arquivo deste aparelho foi escrito por outro PC? (pasta copiada, N6)
	devName := keys.Name(dev, cloudKeyDevice)
	df, ok := byName[devName]
	if !ok {
		// o arquivo deste aparelho já foi escrito daqui e sumiu: outro
		// aparelho tirou este da nuvem
		if !a.cloudHold(gen) {
			return context.Canceled
		}
		if c.f.DevMD5 != "" {
			c.f.Removed = time.Now().UnixMilli()
			a.cloudSaveLocked()
			c.mu.Unlock()
			a.logf("nuvem: outro aparelho tirou este PC da nuvem; os dados daqui continuam")
			return errCloudRemoved
		}
		c.mu.Unlock()
	} else {
		if !a.cloudHold(gen) {
			return context.Canceled
		}
		known, writer := c.f.DevMD5, c.sec.Writer
		c.mu.Unlock()
		if known != "" && df.MD5 != known {
			b, err := d.Get(ctx, df.ID)
			if err != nil {
				return err
			}
			var other rs.Device
			if plain, err := keys.Open(devName, b); err == nil {
				if kind, body, err := rs.Unwrap(plain); err == nil && kind == rs.KindDevice {
					_ = json.Unmarshal(body, &other)
				}
			}
			if other.Writer != "" && other.Writer != writer {
				if a.cloudHold(gen) {
					c.f.Dup, c.f.DupOf = true, other.Name
					a.cloudSaveLocked()
					c.mu.Unlock()
				}
				a.logf("nuvem: outro PC está usando a faixa deste aparelho; a nuvem pausou até você escolher")
				return errCloudDup
			}
		}
	}

	// 1) baixa o que os outros aparelhos subiram de novo
	before, err := a.cloudPacks()
	if err != nil {
		return err
	}
	got, bad, echo, err := a.cloudDownload(ctx, d, files, prefix, gen)
	if err != nil {
		return err
	}

	// 2) sobe o que mudou aqui
	packs := before
	if got > 0 {
		if packs, err = a.cloudPacks(); err != nil {
			return err
		}
	}
	if !a.cloudHold(gen) {
		return context.Canceled
	}
	if c.f.Up == nil {
		c.f.Up = map[string]string{}
	}
	a.cloudEchoLocked(before, packs, echo)
	var todo []cloudPack
	want := map[string]bool{devName: true}
	for _, p := range packs {
		name := keys.Name(dev, p.key)
		want[name] = true
		up := c.f.Up[p.key]
		_, there := byName[name]
		if up == cloudEcho+p.digest || (up == p.digest && there) {
			continue
		}
		todo = append(todo, p)
	}
	c.f.Queue = cloudQueue(todo)
	c.sendN = len(todo)
	c.mu.Unlock()
	sent := 0
	for i, p := range todo {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !a.cloudHold(gen) {
			return context.Canceled
		}
		c.sendI, c.sendNow = i+1, p.month
		c.mu.Unlock()
		a.emitCloud()
		data, err := a.cloudPackData(p)
		if err != nil {
			if p.att != nil {
				a.logf("nuvem: um anexo não subiu (%v)", err)
				continue
			}
			return err
		}
		name := keys.Name(dev, p.key)
		f, err := a.cloudPut(ctx, d, folder, keys, byName, name, data)
		if err != nil {
			return err
		}
		byName[f.Name] = f
		sent++
		if !a.cloudHold(gen) {
			return context.Canceled
		}
		c.f.Up[p.key] = p.digest
		c.f.LastUp = time.Now().UnixMilli()
		c.f.Queue = cloudQueue(todo[i+1:])
		a.cloudSaveLocked()
		c.mu.Unlock()
	}

	// 3) tira da nuvem o que deste aparelho não existe mais (anexo apagado)
	for name, f := range byName {
		if !strings.HasPrefix(name, prefix) || want[name] {
			continue
		}
		if err := d.Delete(ctx, f.ID); err != nil {
			return err
		}
		delete(byName, name)
	}
	if !a.cloudHold(gen) {
		return context.Canceled
	}
	keep := map[string]bool{}
	for _, p := range packs {
		keep[p.key] = true
	}
	for k := range c.f.Up {
		if !keep[k] && k != cloudKeyDevice {
			delete(c.f.Up, k)
		}
	}

	// 4) o arquivo deste aparelho: nome, último envio e quem escreve
	me := rs.Device{ID: dev, Name: c.f.Name, LastUp: c.f.LastUp, Writer: c.sec.Writer, Seq: c.f.Seq + 1}
	devDigest := me.Name + "|" + strconv.FormatInt(me.LastUp, 10) + "|" + me.Writer
	_, there := byName[devName]
	devUp := c.f.Up[cloudKeyDevice] == devDigest && there && c.f.DevMD5 != ""
	c.mu.Unlock()
	if !devUp {
		b, _ := json.Marshal(me)
		f, err := a.cloudPut(ctx, d, folder, keys, byName, devName, rs.Wrap(rs.KindDevice, b))
		if err != nil {
			return err
		}
		byName[f.Name] = f
		if !a.cloudHold(gen) {
			return context.Canceled
		}
		c.f.Up[cloudKeyDevice], c.f.DevMD5, c.f.Seq = devDigest, f.MD5, me.Seq
		c.mu.Unlock()
	}

	var used int64
	for _, f := range byName {
		used += f.Size
	}
	if !a.cloudHold(gen) {
		return context.Canceled
	}
	// aparelhos que saíram da nuvem (removidos em outro PC)
	for id := range c.f.Devs {
		if _, ok := byName[keys.Name(id, cloudKeyDevice)]; !ok {
			delete(c.f.Devs, id)
		}
	}
	c.f.LastSync, c.f.Used, c.f.Queue = time.Now().UnixMilli(), used, nil
	a.cloudSaveLocked()
	c.mu.Unlock()
	if sent > 0 || got > 0 {
		a.logf("nuvem: subiu %d arquivo(s), juntou %d", sent, got)
	}
	if bad > 0 {
		// o resto sincronizou; os que não entraram são tentados de novo (N2)
		return fmt.Errorf("%d pacote(s) de outro aparelho não entraram neste PC; o resto sincronizou e eles são tentados de novo", bad)
	}
	return nil
}

// cloudEchoLocked marca como "já na nuvem" os dias e meses que mudaram só
// pelo que chegou de outro aparelho: eles já estão lá pelo aparelho que
// mandou. Hoje e ontem (UTC) e o mês corrente sobem de novo, porque podem ter
// ganhado dados daqui no meio da mescla.
func (a *App) cloudEchoLocked(before, after []cloudPack, got map[string]bool) {
	c := &a.cloud
	for k := range got {
		c.f.Up[k] = cloudEcho + "1" // anexo que chegou
	}
	pre := map[string]string{}
	for _, p := range before {
		pre[p.key] = p.digest
	}
	now := time.Now().UTC()
	recent := map[string]bool{
		"dia|" + now.Format("2006-01-02"):                   true,
		"dia|" + now.AddDate(0, 0, -1).Format("2006-01-02"): true,
		"espectros|" + now.Format("2006-01"):                true,
	}
	for _, p := range after {
		if (p.kind != rs.KindDay && p.kind != rs.KindSpectra) || recent[p.key] {
			continue
		}
		old, had := pre[p.key]
		if old == p.digest {
			continue
		}
		up := c.f.Up[p.key]
		if !had || up == old || up == cloudEcho+old {
			c.f.Up[p.key] = cloudEcho + p.digest
		}
	}
}

func (a *App) cloudSeen(gen int, name, md5 string) {
	if !a.cloudHold(gen) {
		return
	}
	if a.cloud.f.Seen == nil {
		a.cloud.f.Seen = map[string]string{}
	}
	a.cloud.f.Seen[name] = md5
	a.cloudSaveLocked()
	a.cloud.mu.Unlock()
}

// cloudLater é a marca de um arquivo de uma versão mais nova do formato: é
// tentado de novo quando o app atualizar (A2).
func cloudLater(md5 string) string { return fmt.Sprintf("v%d:%s", rs.FormatVersion, md5) }

// cloudPut cifra e sobe (troca o conteúdo se o arquivo já existe).
func (a *App) cloudPut(ctx context.Context, d *rs.Drive, folder string, keys *rs.Keys, byName map[string]rs.File, name string, data []byte) (rs.File, error) {
	sealed, err := keys.Seal(name, data)
	if err != nil {
		return rs.File{}, err
	}
	id := byName[name].ID
	f, err := d.Put(ctx, folder, id, name, sealed)
	if errors.Is(err, rs.ErrNotFound) && id != "" {
		f, err = d.Put(ctx, folder, "", name, sealed)
	}
	if err == nil && f.Name == "" {
		f.Name = name
	}
	if err == nil && f.MD5 == "" {
		sum := md5.Sum(sealed)
		f.MD5 = hex.EncodeToString(sum[:])
	}
	return f, err
}

// cloudDownload baixa e junta os arquivos novos dos outros aparelhos: os
// aparelhos e o comum primeiro (as medições, para os anexos acharem a
// delas), depois os dias e espectros, por fim os anexos. Um pacote que não
// junta fica de fora desta vez e não para o resto (N2). Devolve quantos
// juntou, quantos não entraram e os anexos que chegaram.
func (a *App) cloudDownload(ctx context.Context, d *rs.Drive, files []rs.File, prefix string, gen int) (got, bad int, echo map[string]bool, err error) {
	c := &a.cloud
	echo = map[string]bool{}
	if !a.cloudHold(gen) {
		return 0, 0, echo, context.Canceled
	}
	keys := c.keys
	seen := map[string]string{}
	for k, v := range c.f.Seen {
		seen[k] = v
	}
	c.mu.Unlock()
	dir := filepath.Join(a.dataDir, "tmp")
	_ = os.MkdirAll(dir, 0o755)
	type item struct {
		name, md5 string
		kind      byte
		body      []byte // comum e anexo
		path      string // dia e espectros
	}
	var items []item
	defer func() {
		for _, it := range items {
			if it.path != "" {
				_ = os.Remove(it.path)
			}
		}
	}()
	newer := 0
	for _, f := range files {
		if f.Name == rs.MetaFile || strings.HasPrefix(f.Name, prefix) {
			continue
		}
		if f.MD5 != "" && (seen[f.Name] == f.MD5 || seen[f.Name] == cloudLater(f.MD5)) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return got, bad, echo, err
		}
		b, err := d.Get(ctx, f.ID)
		if errors.Is(err, rs.ErrNotFound) {
			continue
		}
		if errors.Is(err, rs.ErrTooBig) {
			// nenhum pacote do app chega perto do limite (N14): pula de vez
			a.logf("nuvem: um arquivo da nuvem é grande demais (%d MB) e foi pulado", f.Size>>20)
			a.cloudSeen(gen, f.Name, f.MD5)
			continue
		}
		if err != nil {
			return got, bad, echo, err
		}
		plain, err := keys.Open(f.Name, b)
		if err != nil {
			a.cloudSeen(gen, f.Name, f.MD5) // não é da nuvem (ou estragou): não tenta de novo
			continue
		}
		kind, body, err := rs.Unwrap(plain)
		if errors.Is(err, rs.ErrNewer) {
			newer++
			a.cloudSeen(gen, f.Name, cloudLater(f.MD5))
			continue
		}
		if err != nil {
			a.cloudSeen(gen, f.Name, f.MD5)
			continue
		}
		switch kind {
		case rs.KindDevice:
			var dv rs.Device
			if json.Unmarshal(body, &dv) == nil && dv.ID != "" && a.cloudHold(gen) {
				if c.f.Devs == nil {
					c.f.Devs = map[string]cloudDev{}
				}
				c.f.Devs[dv.ID] = cloudDev{Name: dv.Name, LastUp: dv.LastUp}
				c.mu.Unlock()
			}
			a.cloudSeen(gen, f.Name, f.MD5)
		case rs.KindCommon, rs.KindAttachment:
			items = append(items, item{name: f.Name, md5: f.MD5, kind: kind, body: body})
		case rs.KindDay, rs.KindSpectra:
			tf, err := os.CreateTemp(dir, "nuvem-*.zip")
			if err != nil {
				return got, bad, echo, err
			}
			_, werr := tf.Write(body)
			tf.Close()
			items = append(items, item{name: f.Name, md5: f.MD5, kind: kind, path: tf.Name()})
			if werr != nil {
				return got, bad, echo, werr
			}
		default:
			// tipo que este app não conhece: de uma versão mais nova
			newer++
			a.cloudSeen(gen, f.Name, cloudLater(f.MD5))
		}
	}
	if newer > 0 {
		a.logf("nuvem: %d arquivo(s) de uma versão mais nova do RadLog ficaram para depois de atualizar", newer)
	}
	order := map[byte]int{rs.KindCommon: 0, rs.KindDay: 1, rs.KindSpectra: 1, rs.KindAttachment: 2}
	sort.SliceStable(items, func(i, j int) bool { return order[items[i].kind] < order[items[j].kind] })
	for _, it := range items {
		if err := ctx.Err(); err != nil {
			return got, bad, echo, err
		}
		var err error
		switch it.kind {
		case rs.KindCommon:
			_, err = a.mergeDelta(it.body, "nuvem")
		case rs.KindDay, rs.KindSpectra:
			_, err = a.mergeZip(it.path, false)
		case rs.KindAttachment:
			var key string
			var wait bool
			key, wait, err = a.cloudAttachIn(it.body)
			if wait {
				continue // a medição ainda não chegou: tenta na próxima
			}
			if key != "" {
				echo[key] = true
			}
		}
		if errors.Is(err, rs.ErrFormat) {
			a.cloudSeen(gen, it.name, it.md5)
			continue
		}
		if err != nil {
			a.logf("nuvem: um pacote de outro aparelho não entrou (%v)", err)
			bad++
			continue
		}
		got++
		a.cloudSeen(gen, it.name, it.md5)
	}
	return got, bad, echo, nil
}

// cloudAttachIn grava um anexo que chegou na medição daqui (achada pelo UID
// ou pelo início). wait = a medição ainda não está aqui.
func (a *App) cloudAttachIn(body []byte) (key string, wait bool, err error) {
	h, data, err := rs.UnwrapAttachment(body)
	if err != nil || safeFileName(h.File) != h.File {
		return "", false, rs.ErrFormat
	}
	a.mu.Lock()
	a.tombsLoadLocked()
	var id int64
	found, listed := false, false
	uid := h.UID
	for _, m := range a.measurements {
		if (h.UID != "" && m.UID == h.UID) || (h.UID == "" && m.Time/1000 == h.Start) {
			found, id, uid = true, m.ID, m.UID
			for _, at := range m.Attachments {
				listed = listed || at.File == h.File
			}
			break
		}
	}
	gone := h.UID != "" && a.tombed(recMeasure, h.UID)
	a.mu.Unlock()
	if !found {
		return "", !gone, nil
	}
	if !listed {
		return "", false, nil // tirado da medição depois
	}
	dst := filepath.Join(a.attachDir(id), h.File)
	if _, err := os.Stat(dst); err != nil {
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return "", false, err
		}
		if err := writeFileAtomic(dst, data, 0o644); err != nil {
			return "", false, err
		}
	}
	return "anexo|" + uid + "/" + h.File, false, nil
}
